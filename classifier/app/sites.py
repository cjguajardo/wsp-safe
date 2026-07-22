from __future__ import annotations

import ipaddress
import json
import re
import socket
import sqlite3
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, replace
from pathlib import Path
from typing import Callable, Iterable, Protocol
from urllib.error import HTTPError
from urllib.parse import urlencode, urlsplit, urlunsplit
from urllib.request import Request, urlopen


IPQS_ENDPOINT = "https://ipqualityscore.com/api/json/url"
CLOUDFLARE_ENDPOINT = "https://api.cloudflare.com/client/v4/accounts"
URL_PATTERN = re.compile(r"(?i)(?:https?://|www\.)[^\s<>\"']+")
TRAILING_URL_PUNCTUATION = ".,;:!?)]}»”'"
NSFW_CATEGORIES = {
    "adult",
    "adult themes",
    "nudity",
    "pornography",
    "sexual content",
}


@dataclass(frozen=True)
class Classification:
    domain: str
    categories: tuple[str, ...]
    nsfw: bool
    score: float
    provider: str
    cached: bool = False


@dataclass(frozen=True)
class SiteTextAnalysis:
    results: tuple[Classification, ...]
    score: float
    uncertain: bool


class SiteProvider(Protocol):
    name: str

    def classify(self, url: str) -> Classification:
        ...


def classify_text_sites(
    text: str,
    gateway: "RotatingSiteGateway",
    max_urls: int,
) -> SiteTextAnalysis:
    if max_urls < 1:
        raise ValueError("el máximo de URLs debe ser positivo")
    results: list[Classification] = []
    uncertain = False
    for url in extract_urls(text)[:max_urls]:
        try:
            results.append(gateway.classify(url))
        except (OSError, RuntimeError, ValueError):
            uncertain = True
    return SiteTextAnalysis(
        results=tuple(results),
        score=max((result.score for result in results if result.nsfw), default=0.0),
        uncertain=uncertain,
    )


def normalize_site_url(value: str) -> str:
    candidate = value.strip()
    if candidate.lower().startswith("www."):
        candidate = "https://" + candidate
    parsed = urlsplit(candidate)
    if parsed.scheme.lower() not in {"http", "https"} or not parsed.hostname:
        raise ValueError("la URL debe usar HTTP o HTTPS")
    if parsed.username is not None or parsed.password is not None:
        raise ValueError("la URL no puede incluir credenciales")

    try:
        hostname = parsed.hostname.encode("idna").decode("ascii").lower().rstrip(".")
        port = parsed.port
    except (UnicodeError, ValueError) as error:
        raise ValueError("la URL contiene un dominio o puerto inválido") from error
    if not hostname:
        raise ValueError("la URL no contiene un dominio")

    if ":" in hostname:
        hostname = f"[{hostname}]"
    default_port = (parsed.scheme.lower() == "http" and port == 80) or (
        parsed.scheme.lower() == "https" and port == 443
    )
    authority = hostname if port is None or default_port else f"{hostname}:{port}"
    return urlunsplit((
        parsed.scheme.lower(),
        authority,
        parsed.path or "/",
        parsed.query,
        "",
    ))


def domain_from_url(value: str) -> str:
    normalized = normalize_site_url(value)
    hostname = urlsplit(normalized).hostname
    if hostname is None:
        raise ValueError("la URL no contiene un dominio")
    return hostname.lower().rstrip(".")


def extract_urls(text: str) -> list[str]:
    found: list[str] = []
    seen: set[str] = set()
    for match in URL_PATTERN.finditer(text):
        candidate = match.group(0).rstrip(TRAILING_URL_PUNCTUATION)
        try:
            normalized = normalize_site_url(candidate)
        except ValueError:
            continue
        if normalized not in seen:
            seen.add(normalized)
            found.append(normalized)
    return found


def resolve_addresses(hostname: str) -> list[ipaddress.IPv4Address | ipaddress.IPv6Address]:
    addresses = []
    for record in socket.getaddrinfo(hostname, None, type=socket.SOCK_STREAM):
        addresses.append(ipaddress.ip_address(record[4][0]))
    return addresses


def is_public_web_url(
    value: str,
    resolver: Callable[[str], Iterable[ipaddress.IPv4Address | ipaddress.IPv6Address | str]] = resolve_addresses,
) -> bool:
    try:
        normalized = normalize_site_url(value)
        hostname = domain_from_url(normalized)
    except ValueError:
        return False
    lowered = hostname.lower()
    if lowered == "localhost" or lowered.endswith((
        ".localhost",
        ".local",
        ".internal",
        ".home",
        ".lan",
    )):
        return False
    try:
        literal = ipaddress.ip_address(hostname)
        addresses = [literal]
    except ValueError:
        try:
            addresses = [ipaddress.ip_address(address) for address in resolver(hostname)]
        except (OSError, ValueError):
            return False
    if not addresses:
        return False
    return all(address.is_global for address in addresses)


class DomainCategoryStore:
    def __init__(self, database_path: str) -> None:
        self._database_path = database_path
        Path(database_path).parent.mkdir(parents=True, exist_ok=True)
        with self._connect() as connection:
            connection.execute("""
                CREATE TABLE IF NOT EXISTS nsfw_domains (
                    domain TEXT PRIMARY KEY,
                    categories_json TEXT NOT NULL,
                    score REAL NOT NULL CHECK (score >= 0 AND score <= 1),
                    provider TEXT NOT NULL,
                    checked_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
                )
            """)

    def _connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self._database_path, timeout=10)
        connection.execute("PRAGMA busy_timeout = 10000")
        return connection

    def find_nsfw(self, domain: str) -> Classification | None:
        with self._connect() as connection:
            row = connection.execute(
                "SELECT categories_json, score, provider FROM nsfw_domains WHERE domain = ?",
                (domain.lower().rstrip("."),),
            ).fetchone()
        if row is None:
            return None
        categories = tuple(str(value) for value in json.loads(row[0]))
        return Classification(
            domain=domain.lower().rstrip("."),
            categories=categories,
            nsfw=True,
            score=float(row[1]),
            provider=str(row[2]),
            cached=True,
        )

    def save_nsfw(self, result: Classification) -> None:
        if not result.nsfw:
            return
        with self._connect() as connection:
            connection.execute("""
                INSERT INTO nsfw_domains (domain, categories_json, score, provider, checked_at)
                VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
                ON CONFLICT(domain) DO UPDATE SET
                    categories_json = excluded.categories_json,
                    score = MAX(nsfw_domains.score, excluded.score),
                    provider = excluded.provider,
                    checked_at = CURRENT_TIMESTAMP
            """, (
                result.domain.lower().rstrip("."),
                json.dumps(result.categories, ensure_ascii=False),
                min(max(float(result.score), 0.0), 1.0),
                result.provider,
            ))


class RotatingSiteGateway:
    def __init__(self, store: DomainCategoryStore, providers: list[SiteProvider]) -> None:
        if not providers:
            raise ValueError("se requiere al menos un verificador de sitios")
        self._store = store
        self._providers = list(providers)
        self._next_provider = 0
        self._rotation_lock = threading.Lock()

    def classify(self, url: str) -> Classification:
        normalized = normalize_site_url(url)
        domain = domain_from_url(normalized)
        cached = self._store.find_nsfw(domain)
        if cached is not None:
            return cached

        with self._rotation_lock:
            start = self._next_provider
            self._next_provider = (self._next_provider + 1) % len(self._providers)

        errors: list[str] = []
        for offset in range(len(self._providers)):
            provider = self._providers[(start + offset) % len(self._providers)]
            try:
                result = replace(provider.classify(normalized), domain=domain, cached=False)
            except (OSError, RuntimeError, ValueError) as error:
                errors.append(f"{provider.name}: {error}")
                continue
            if result.nsfw:
                self._store.save_nsfw(result)
            return result
        raise RuntimeError("ningún verificador pudo clasificar el sitio: " + "; ".join(errors))

    def close(self) -> None:
        for provider in self._providers:
            close = getattr(provider, "close", None)
            if callable(close):
                close()


class IPQualityScoreProvider:
    name = "ipqs"

    def __init__(self, api_key: str, timeout_seconds: float = 10.0) -> None:
        if not api_key.strip():
            raise ValueError("IPQS requiere una clave")
        self._api_key = api_key.strip()
        self._timeout_seconds = timeout_seconds

    def classify(self, url: str) -> Classification:
        normalized = normalize_site_url(url)
        body = urlencode({
            "url": normalized,
            "strictness": "0",
            "fast": "false",
            "timeout": str(max(1, min(int(self._timeout_seconds), 10))),
        }).encode("utf-8")
        request = Request(
            IPQS_ENDPOINT,
            data=body,
            headers={
                "Content-Type": "application/x-www-form-urlencoded",
                "IPQS-KEY": self._api_key,
            },
            method="POST",
        )
        with urlopen(request, timeout=self._timeout_seconds + 2) as response:
            payload = json.loads(response.read())
        if not payload.get("success"):
            raise RuntimeError(str(payload.get("message", "IPQS rechazó la consulta")))
        adult = bool(payload.get("adult"))
        category = str(payload.get("category", "")).strip()
        categories = tuple(value for value in (category, "adult" if adult else "") if value)
        return Classification(
            domain=domain_from_url(normalized),
            categories=categories,
            nsfw=adult,
            score=1.0 if adult else 0.0,
            provider=self.name,
        )


class CloudflareRadarProvider:
    name = "cloudflare"

    def __init__(
        self,
        account_id: str,
        api_token: str,
        timeout_seconds: float = 45.0,
        poll_interval_seconds: float = 10.0,
    ) -> None:
        if not account_id.strip() or not api_token.strip():
            raise ValueError("Cloudflare Radar requiere cuenta y token")
        self._account_id = account_id.strip()
        self._api_token = api_token.strip()
        self._timeout_seconds = timeout_seconds
        self._poll_interval_seconds = poll_interval_seconds

    def _request(self, endpoint: str, *, body: dict[str, object] | None = None) -> dict[str, object]:
        encoded = None if body is None else json.dumps(body).encode("utf-8")
        request = Request(
            endpoint,
            data=encoded,
            headers={
                "Authorization": f"Bearer {self._api_token}",
                "Content-Type": "application/json",
            },
            method="POST" if body is not None else "GET",
        )
        with urlopen(request, timeout=min(self._timeout_seconds, 15.0)) as response:
            payload = json.loads(response.read())
        if payload.get("success") is False:
            raise RuntimeError("Cloudflare Radar rechazó la consulta")
        result = payload.get("result", payload)
        if not isinstance(result, dict):
            raise RuntimeError("Cloudflare Radar devolvió una respuesta inválida")
        return result

    def classify(self, url: str) -> Classification:
        normalized = normalize_site_url(url)
        base = f"{CLOUDFLARE_ENDPOINT}/{self._account_id}/urlscanner/v2"
        submission = self._request(
            f"{base}/scan",
            body={"url": normalized, "visibility": "Unlisted"},
        )
        scan_id = str(submission.get("uuid", ""))
        if not scan_id:
            raise RuntimeError("Cloudflare Radar no entregó un identificador de escaneo")

        deadline = time.monotonic() + self._timeout_seconds
        while True:
            try:
                report = self._request(f"{base}/result/{scan_id}")
                break
            except HTTPError as error:
                if error.code not in {404, 425} or time.monotonic() >= deadline:
                    raise RuntimeError(f"Cloudflare Radar devolvió HTTP {error.code}") from error
            if time.monotonic() >= deadline:
                raise RuntimeError("Cloudflare Radar agotó el tiempo de espera")
            time.sleep(self._poll_interval_seconds)

        categories = _cloudflare_categories(report)
        nsfw = any(category.casefold() in NSFW_CATEGORIES for category in categories)
        return Classification(
            domain=domain_from_url(normalized),
            categories=categories,
            nsfw=nsfw,
            score=1.0 if nsfw else 0.0,
            provider=self.name,
        )


def _cloudflare_categories(report: dict[str, object]) -> tuple[str, ...]:
    processors = report.get("meta", {})
    if not isinstance(processors, dict):
        return ()
    processors = processors.get("processors", {})
    if not isinstance(processors, dict):
        return ()

    names: list[str] = []
    for key in ("domainCategories", "urlCategories"):
        section = processors.get(key, {})
        if isinstance(section, dict):
            _collect_category_names(section.get("data", []), names)
    return tuple(dict.fromkeys(names))


def _collect_category_names(value: object, names: list[str]) -> None:
    if isinstance(value, list):
        for item in value:
            _collect_category_names(item, names)
    elif isinstance(value, dict):
        name = value.get("name")
        if isinstance(name, str) and name.strip():
            names.append(name.strip())
        for key in ("content", "risks"):
            if key in value:
                _collect_category_names(value[key], names)


class PlaywrightScreenshotProvider:
    name = "playwright"

    def __init__(
        self,
        scorer: Callable[[bytes], float],
        threshold: float,
        timeout_seconds: float = 20.0,
        resolver: Callable[[str], Iterable[ipaddress.IPv4Address | ipaddress.IPv6Address | str]] = resolve_addresses,
    ) -> None:
        if threshold <= 0 or threshold > 1:
            raise ValueError("el umbral de Playwright debe estar entre cero y uno")
        self._scorer = scorer
        self._threshold = threshold
        self._timeout_seconds = timeout_seconds
        self._resolver = resolver
        self._executor = ThreadPoolExecutor(max_workers=1, thread_name_prefix="playwright")
        self._playwright = None
        self._browser = None
        self._executor.submit(self._start).result(timeout=timeout_seconds)

    def _start(self) -> None:
        from playwright.sync_api import sync_playwright

        self._playwright = sync_playwright().start()
        self._browser = self._playwright.chromium.launch(
            headless=True,
            args=["--disable-dev-shm-usage", "--no-sandbox"],
        )

    def classify(self, url: str) -> Classification:
        normalized = normalize_site_url(url)
        if not is_public_web_url(normalized, self._resolver):
            raise ValueError("Playwright bloqueó un destino local o privado")
        return self._executor.submit(self._classify, normalized).result(
            timeout=self._timeout_seconds + 5
        )

    def _classify(self, normalized: str) -> Classification:
        if self._browser is None:
            raise RuntimeError("Playwright no está iniciado")
        context = self._browser.new_context(
            accept_downloads=False,
            service_workers="block",
            viewport={"width": 1280, "height": 720},
        )
        page = context.new_page()

        def protect_route(route) -> None:
            requested = route.request.url
            scheme = urlsplit(requested).scheme.lower()
            if scheme in {"data", "blob", "about"}:
                route.continue_()
            elif is_public_web_url(requested, self._resolver):
                route.continue_()
            else:
                route.abort("blockedbyclient")

        page.route("**/*", protect_route)
        try:
            page.goto(
                normalized,
                wait_until="domcontentloaded",
                timeout=self._timeout_seconds * 1000,
            )
            page.wait_for_timeout(1000)
            screenshot = page.screenshot(type="jpeg", quality=75, full_page=False)
            score = min(max(float(self._scorer(screenshot)), 0.0), 1.0)
        finally:
            context.close()
        nsfw = score >= self._threshold
        return Classification(
            domain=domain_from_url(normalized),
            categories=("visual_nsfw",) if nsfw else ("visual_safe",),
            nsfw=nsfw,
            score=score,
            provider=self.name,
        )

    def close(self) -> None:
        def stop() -> None:
            if self._browser is not None:
                self._browser.close()
                self._browser = None
            if self._playwright is not None:
                self._playwright.stop()
                self._playwright = None

        self._executor.submit(stop).result(timeout=10)
        self._executor.shutdown(wait=True)
