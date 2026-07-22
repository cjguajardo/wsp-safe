import unittest

from app.logging import log_classification_result, log_requests_enabled


class LoggingTest(unittest.TestCase):
    def test_env_flag_accepts_common_enabled_values(self):
        self.assertTrue(log_requests_enabled("true"))
        self.assertTrue(log_requests_enabled("1"))
        self.assertTrue(log_requests_enabled("sí"))
        self.assertFalse(log_requests_enabled("false"))
        self.assertFalse(log_requests_enabled(""))

    def test_classification_log_excludes_content(self):
        with self.assertLogs("wsp-safe.classifier", level="INFO") as captured:
            log_classification_result(
                message_id="abc-123",
                sender_id="56911111111@s.whatsapp.net",
                kind="text",
                mime_type="",
                text_length=len("contenido sensible"),
                media_base64_length=len("base64-sensible"),
                sexual_score=0.5,
                sexual_minors_score=0.0,
                uncertain=False,
                duration_ms=12.3,
            )

        output = "\n".join(captured.output)
        self.assertIn("id=abc-123", output)
        self.assertIn("remitente=56911111111@s.whatsapp.net", output)
        self.assertIn("tipo=text", output)
        self.assertIn("texto_caracteres=18", output)
        self.assertIn("media_base64_caracteres=15", output)
        self.assertNotIn("contenido sensible", output)
        self.assertNotIn("base64-sensible", output)


if __name__ == "__main__":
    unittest.main()
