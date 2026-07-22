import unittest

from app.moderation import frame_interval, score_detections, score_text


class ModerationTest(unittest.TestCase):
    def test_explicit_detection_uses_highest_score(self):
        score = score_detections(
            [
                {"class": "FACE_FEMALE", "score": 0.99},
                {"class": "FEMALE_BREAST_EXPOSED", "score": 0.81},
                {"class": "BUTTOCKS_EXPOSED", "score": 0.72},
            ]
        )
        self.assertEqual(score, 0.81)

    def test_suggestive_detection_is_conservatively_weighted(self):
        score = score_detections(
            [{"class": "FEMALE_GENITALIA_COVERED", "score": 0.8}]
        )
        self.assertEqual(score, 0.4)

    def test_safe_detection_returns_zero(self):
        self.assertEqual(
            score_detections([{"class": "FACE_MALE", "score": 0.95}]), 0.0
        )

    def test_text_keywords(self):
        self.assertEqual(score_text("Mira este video porno"), 1.0)
        self.assertEqual(score_text("Nos vemos mañana para almorzar"), 0.0)

    def test_frame_interval_spreads_samples_across_video(self):
        self.assertAlmostEqual(frame_interval(60.0, 6), 60.0 / 7.0)
        self.assertEqual(frame_interval(1.0, 6), 0.5)


if __name__ == "__main__":
    unittest.main()
