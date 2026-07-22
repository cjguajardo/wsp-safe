import re
import unicodedata
from collections.abc import Iterable, Mapping


EXPLICIT_CLASSES = {
    "ANUS_EXPOSED",
    "BUTTOCKS_EXPOSED",
    "FEMALE_BREAST_EXPOSED",
    "FEMALE_GENITALIA_EXPOSED",
    "MALE_GENITALIA_EXPOSED",
}

SUGGESTIVE_CLASSES = {
    "ANUS_COVERED",
    "BUTTOCKS_COVERED",
    "FEMALE_BREAST_COVERED",
    "FEMALE_GENITALIA_COVERED",
}

SEXUAL_TERMS = {
    "desnuda",
    "desnudo",
    "hentai",
    "nude",
    "nudes",
    "onlyfans",
    "porn",
    "porno",
    "pornografia",
    "sex",
    "sexo",
    "sexual",
    "xxx",
}


def score_detections(detections: Iterable[Mapping[str, object]]) -> float:
    """Convert NudeNet object detections into one conservative sexual score."""
    score = 0.0
    for detection in detections:
        label = str(detection.get("class", "")).upper()
        raw_score = detection.get("score", 0.0)
        try:
            confidence = min(max(float(raw_score), 0.0), 1.0)
        except (TypeError, ValueError):
            continue
        if label in EXPLICIT_CLASSES:
            score = max(score, confidence)
        elif label in SUGGESTIVE_CLASSES:
            score = max(score, confidence * 0.5)
    return score


def score_text(text: str) -> float:
    normalized = unicodedata.normalize("NFKD", text.casefold())
    normalized = "".join(character for character in normalized if not unicodedata.combining(character))
    tokens = set(re.findall(r"[a-z0-9]+", normalized))
    return 1.0 if tokens.intersection(SEXUAL_TERMS) else 0.0


def frame_interval(duration_seconds: float, samples: int) -> float:
    if samples < 1:
        raise ValueError("samples must be positive")
    if duration_seconds <= 0:
        return 5.0
    return max(duration_seconds / (samples + 1), 0.5)
