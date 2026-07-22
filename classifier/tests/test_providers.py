import json
import unittest
from unittest.mock import patch

from app.providers import GoogleSafeSearch, likelihood_score, maximum_score


class ProvidersTest(unittest.TestCase):
    def test_maximum_score_combines_all_classifiers(self):
        self.assertEqual(maximum_score(0.2, 0.8, 0.4), 0.8)

    def test_likelihood_score_is_conservative(self):
        self.assertEqual(likelihood_score("VERY_UNLIKELY"), 0.0)
        self.assertEqual(likelihood_score("POSSIBLE"), 0.5)
        self.assertEqual(likelihood_score("VERY_LIKELY"), 1.0)

    def test_google_safe_search_is_disabled_without_key(self):
        provider = GoogleSafeSearch("")
        self.assertFalse(provider.enabled)
        self.assertEqual(provider.score(b"imagen"), 0.0)

    @patch("app.providers.urlopen")
    def test_google_safe_search_uses_adult_and_racy_scores(self, urlopen):
        response = urlopen.return_value.__enter__.return_value
        response.read.return_value = json.dumps({
            "responses": [{"safeSearchAnnotation": {
                "adult": "UNLIKELY",
                "racy": "LIKELY",
            }}]
        }).encode()

        provider = GoogleSafeSearch("clave")

        self.assertEqual(provider.score(b"imagen"), 0.75)
        request = urlopen.call_args.args[0]
        self.assertNotIn("clave", request.full_url)
        self.assertEqual(request.headers["X-goog-api-key"], "clave")


if __name__ == "__main__":
    unittest.main()
