from __future__ import annotations

import ipaddress
import json
import tempfile
import unittest
from pathlib import Path
from urllib.parse import parse_qs
from unittest.mock import MagicMock, patch

from app.sites import (
    CloudflareRadarProvider,
    Classification,
    DomainCategoryStore,
    IPQualityScoreProvider,
    RotatingSiteGateway,
    _cloudflare_categories,
    classify_text_sites,
    extract_urls,
    is_public_web_url,
    normalize_site_url,
)


class StubProvider:
    def __init__(self, name: str, results: list[Classification | Exception]):
        self.name = name
        self.results = list(results)
        self.calls: list[str] = []

    def classify(self, url: str) -> Classification:
        self.calls.append(url)
        result = self.results.pop(0)
        if isinstance(result, Exception):
            raise result
        return result


class SitesTest(unittest.TestCase):
    def test_extracts_and_normalizes_web_urls(self):
        text = "Mira https://Example.COM/ruta?q=1 y también www.ejemplo.cl/video."

        self.assertEqual(
            extract_urls(text),
            ["https://example.com/ruta?q=1", "https://www.ejemplo.cl/video"],
        )

    def test_rejects_non_http_urls_and_credentials(self):
        for value in [
            "file:///etc/passwd",
            "ftp://example.com/archivo",
            "https://usuario:clave@example.com/",
        ]:
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    normalize_site_url(value)

    def test_blocks_local_and_private_destinations(self):
        private_resolver = lambda _host: [ipaddress.ip_address("192.168.1.10")]
        public_resolver = lambda _host: [ipaddress.ip_address("93.184.216.34")]

        self.assertFalse(is_public_web_url("http://localhost", public_resolver))
        self.assertFalse(is_public_web_url("http://127.0.0.1", public_resolver))
        self.assertFalse(is_public_web_url("https://example.com", private_resolver))
        self.assertTrue(is_public_web_url("https://example.com", public_resolver))

    def test_store_persists_nsfw_domain_and_categories(self):
        with tempfile.TemporaryDirectory() as directory:
            store = DomainCategoryStore(str(Path(directory) / "sites.db"))
            result = Classification(
                domain="example.com",
                categories=("Pornography", "Nudity"),
                nsfw=True,
                score=0.92,
                provider="cloudflare",
            )

            store.save_nsfw(result)

            cached = store.find_nsfw("example.com")
            self.assertIsNotNone(cached)
            self.assertTrue(cached.cached)
            self.assertEqual(cached.categories, ("Pornography", "Nudity"))
            self.assertEqual(cached.provider, "cloudflare")

    def test_gateway_uses_cache_before_providers(self):
        with tempfile.TemporaryDirectory() as directory:
            store = DomainCategoryStore(str(Path(directory) / "sites.db"))
            store.save_nsfw(Classification(
                domain="example.com",
                categories=("adult",),
                nsfw=True,
                score=1.0,
                provider="ipqs",
            ))
            provider = StubProvider("playwright", [])
            gateway = RotatingSiteGateway(store, [provider])

            result = gateway.classify("https://example.com/ruta")

            self.assertTrue(result.cached)
            self.assertEqual(provider.calls, [])

    def test_gateway_rotates_provider_for_each_cache_miss(self):
        with tempfile.TemporaryDirectory() as directory:
            store = DomainCategoryStore(str(Path(directory) / "sites.db"))
            first = StubProvider("ipqs", [Classification(
                domain="uno.example",
                categories=("safe",),
                nsfw=False,
                score=0.0,
                provider="ipqs",
            )])
            second = StubProvider("cloudflare", [Classification(
                domain="dos.example",
                categories=("safe",),
                nsfw=False,
                score=0.0,
                provider="cloudflare",
            )])
            gateway = RotatingSiteGateway(store, [first, second])

            self.assertEqual(gateway.classify("https://uno.example").provider, "ipqs")
            self.assertEqual(gateway.classify("https://dos.example").provider, "cloudflare")

    def test_gateway_falls_back_after_provider_error(self):
        with tempfile.TemporaryDirectory() as directory:
            store = DomainCategoryStore(str(Path(directory) / "sites.db"))
            failing = StubProvider("ipqs", [RuntimeError("sin cuota")])
            fallback = StubProvider("playwright", [Classification(
                domain="example.com",
                categories=("visual_nsfw",),
                nsfw=True,
                score=0.84,
                provider="playwright",
            )])
            gateway = RotatingSiteGateway(store, [failing, fallback])

            result = gateway.classify("https://example.com")

            self.assertTrue(result.nsfw)
            self.assertEqual(result.provider, "playwright")
            self.assertIsNotNone(store.find_nsfw("example.com"))

    def test_safe_results_are_not_persisted(self):
        with tempfile.TemporaryDirectory() as directory:
            store = DomainCategoryStore(str(Path(directory) / "sites.db"))
            provider = StubProvider("ipqs", [Classification(
                domain="example.com",
                categories=("News",),
                nsfw=False,
                score=0.0,
                provider="ipqs",
            )])
            gateway = RotatingSiteGateway(store, [provider])

            gateway.classify("https://example.com")

            self.assertIsNone(store.find_nsfw("example.com"))

    @patch("app.sites.urlopen")
    def test_ipqs_keeps_key_out_of_url_and_reads_adult_flag(self, urlopen):
        response = urlopen.return_value.__enter__.return_value
        response.read.return_value = json.dumps({
            "success": True,
            "adult": True,
            "category": "Adult Content",
        }).encode()
        provider = IPQualityScoreProvider("secreto")

        result = provider.classify("https://example.com/video")

        request = urlopen.call_args.args[0]
        self.assertNotIn("secreto", request.full_url)
        self.assertEqual(request.headers["Ipqs-key"], "secreto")
        self.assertEqual(parse_qs(request.data.decode())["url"], ["https://example.com/video"])
        self.assertTrue(result.nsfw)
        self.assertIn("Adult Content", result.categories)

    def test_reads_cloudflare_domain_and_url_categories(self):
        categories = _cloudflare_categories({
            "meta": {"processors": {
                "domainCategories": {"data": [{"name": "Social Networks"}]},
                "urlCategories": {"data": [{
                    "content": [{"name": "Pornography"}, {"name": "Nudity"}],
                    "risks": [],
                }]},
            }}
        })

        self.assertEqual(categories, ("Social Networks", "Pornography", "Nudity"))

    @patch("app.sites.urlopen")
    def test_cloudflare_submits_unlisted_scan_and_detects_nsfw(self, urlopen):
        submission = MagicMock()
        submission.__enter__.return_value.read.return_value = json.dumps({
            "result": {"uuid": "scan-123"},
            "success": True,
        }).encode()
        report = MagicMock()
        report.__enter__.return_value.read.return_value = json.dumps({
            "result": {
                "meta": {"processors": {
                    "domainCategories": {"data": [{"name": "Pornography"}]}
                }}
            },
            "success": True,
        }).encode()
        urlopen.side_effect = [submission, report]
        provider = CloudflareRadarProvider(
            "cuenta",
            "token",
            timeout_seconds=1,
            poll_interval_seconds=0,
        )

        result = provider.classify("https://example.com")

        create_request = urlopen.call_args_list[0].args[0]
        self.assertEqual(json.loads(create_request.data)["visibility"], "Unlisted")
        self.assertEqual(create_request.headers["Authorization"], "Bearer token")
        self.assertTrue(result.nsfw)
        self.assertEqual(result.score, 1.0)

    def test_text_site_analysis_combines_scores_and_marks_failures_uncertain(self):
        class Gateway:
            def classify(self, url: str) -> Classification:
                if "falla" in url:
                    raise RuntimeError("falló")
                return Classification(
                    domain="adult.example",
                    categories=("visual_nsfw",),
                    nsfw=True,
                    score=0.88,
                    provider="playwright",
                )

        analysis = classify_text_sites(
            "https://adult.example/video https://falla.example",
            Gateway(),
            max_urls=3,
        )

        self.assertEqual(analysis.score, 0.88)
        self.assertTrue(analysis.uncertain)
        self.assertEqual(len(analysis.results), 1)


if __name__ == "__main__":
    unittest.main()
