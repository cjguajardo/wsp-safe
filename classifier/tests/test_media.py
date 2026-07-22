import base64
import unittest

from app.media import decode_media


class MediaTest(unittest.TestCase):
    def test_decodes_valid_base64(self):
        encoded = base64.b64encode(b"image").decode("ascii")
        self.assertEqual(decode_media(encoded, 10), b"image")

    def test_rejects_invalid_base64(self):
        with self.assertRaises(ValueError):
            decode_media("not base64!", 100)

    def test_rejects_oversized_media_before_and_after_decode(self):
        encoded = base64.b64encode(b"too large").decode("ascii")
        with self.assertRaises(ValueError):
            decode_media(encoded, 4)


if __name__ == "__main__":
    unittest.main()
