import base64
import io
import json
import threading
from typing import Iterable
from urllib.request import Request, urlopen


GOOGLE_VISION_ENDPOINT = "https://vision.googleapis.com/v1/images:annotate"


def maximum_score(*scores: float) -> float:
    return max((min(max(float(score), 0.0), 1.0) for score in scores), default=0.0)


def likelihood_score(likelihood: str) -> float:
    return {
        "UNKNOWN": 0.5,
        "VERY_UNLIKELY": 0.0,
        "UNLIKELY": 0.25,
        "POSSIBLE": 0.5,
        "LIKELY": 0.75,
        "VERY_LIKELY": 1.0,
    }.get(str(likelihood).upper(), 0.5)


class OpenNSFW2Classifier:
    def __init__(self) -> None:
        import opennsfw2

        self._module = opennsfw2
        self._model = opennsfw2.make_open_nsfw_model()
        self._lock = threading.Lock()

    def score(self, media: bytes) -> float:
        import numpy as np
        from PIL import Image

        with Image.open(io.BytesIO(media)) as image:
            prepared = self._module.preprocess_image(
                image.convert("RGB"),
                self._module.Preprocessing.YAHOO,
            )
        inputs = np.expand_dims(prepared, axis=0)
        with self._lock:
            predictions = self._model.predict(inputs, verbose=0)
        return float(predictions[0][1])

    def score_many(self, images: Iterable[bytes]) -> float:
        return maximum_score(*(self.score(image) for image in images))


class GoogleSafeSearch:
    def __init__(self, api_key: str, timeout_seconds: float = 15.0) -> None:
        self._api_key = api_key.strip()
        self._timeout_seconds = timeout_seconds

    @property
    def enabled(self) -> bool:
        return bool(self._api_key)

    def score(self, media: bytes) -> float:
        if not self.enabled:
            return 0.0
        body = json.dumps({
            "requests": [{
                "image": {"content": base64.b64encode(media).decode("ascii")},
                "features": [{"type": "SAFE_SEARCH_DETECTION"}],
            }]
        }).encode("utf-8")
        request = Request(
            GOOGLE_VISION_ENDPOINT,
            data=body,
            headers={
                "Content-Type": "application/json",
                "X-goog-api-key": self._api_key,
            },
            method="POST",
        )
        with urlopen(request, timeout=self._timeout_seconds) as response:
            payload = json.loads(response.read())
        responses = payload.get("responses", [])
        if not responses:
            raise RuntimeError("Google SafeSearch returned no response")
        first = responses[0]
        if first.get("error"):
            raise RuntimeError("Google SafeSearch returned an error")
        annotation = first.get("safeSearchAnnotation", {})
        return maximum_score(
            likelihood_score(annotation.get("adult", "UNKNOWN")),
            likelihood_score(annotation.get("racy", "UNKNOWN")),
        )
