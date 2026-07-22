import logging
import time
from typing import Any


logger = logging.getLogger("wsp-safe.classifier")
logger.setLevel(logging.INFO)


def log_requests_enabled(value: str) -> bool:
    return value.strip().lower() in {"1", "true", "yes", "on", "sí", "si"}


def request_started_at() -> float:
    return time.perf_counter()


def request_duration_ms(started_at: float) -> float:
    return (time.perf_counter() - started_at) * 1000


def log_http_request(request: Any, response: Any, duration_ms: float) -> None:
    logger.info(
        "petición al clasificador: método=%s ruta=%s estado=%s duración_ms=%.1f",
        request.method,
        request.url.path,
        response.status_code,
        duration_ms,
    )


def log_classification_result(
    *,
    message_id: str,
    sender_id: str,
    kind: str,
    mime_type: str,
    text_length: int,
    media_base64_length: int,
    sexual_score: float,
    sexual_minors_score: float,
    uncertain: bool,
    duration_ms: float,
) -> None:
    logger.info(
        (
            "clasificación completada: id=%s remitente=%s tipo=%s mime=%s texto_caracteres=%s "
            "media_base64_caracteres=%s puntuación_sexual=%.3f "
            "puntuación_sexual_menores=%.3f dudoso=%s duración_ms=%.1f"
        ),
        message_id,
        sender_id or "desconocido",
        kind,
        mime_type or "sin-mime",
        text_length,
        media_base64_length,
        sexual_score,
        sexual_minors_score,
        uncertain,
        duration_ms,
    )
