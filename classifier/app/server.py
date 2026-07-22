import os
import secrets
import subprocess
import tempfile
import threading
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Literal

from fastapi import FastAPI, Header, HTTPException, Request, status
from nudenet import NudeDetector
from pydantic import BaseModel, Field

from app.media import decode_media, extract_frames
from app.moderation import score_detections, score_text


MAX_MEDIA_BYTES = int(os.getenv("CLASSIFIER_MAX_MEDIA_BYTES", str(20 << 20)))
VIDEO_SAMPLES = int(os.getenv("CLASSIFIER_VIDEO_SAMPLES", "6"))
CLASSIFIER_TOKEN = os.getenv("CLASSIFIER_TOKEN", "")
detector_lock = threading.Lock()


class ClassifyRequest(BaseModel):
    message_id: str = Field(min_length=1, max_length=256)
    kind: Literal["text", "image", "video", "sticker"]
    mime_type: str = Field(default="", max_length=128)
    text: str = Field(default="", max_length=100_000)
    media_base64: str = ""


class ClassifyResponse(BaseModel):
    sexual_score: float
    sexual_minors_score: float = 0.0
    uncertain: bool = False


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.detector = NudeDetector()
    yield


app = FastAPI(
    title="wsp-safe NudeNet classifier",
    docs_url=None,
    redoc_url=None,
    openapi_url=None,
    lifespan=lifespan,
)


def require_token(authorization: str | None) -> None:
    if not CLASSIFIER_TOKEN:
        return
    expected = f"Bearer {CLASSIFIER_TOKEN}"
    if authorization is None or not secrets.compare_digest(authorization, expected):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized")


@app.get("/healthz")
def health(request: Request) -> dict[str, str]:
    if not hasattr(request.app.state, "detector"):
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="loading")
    return {"status": "ok"}


@app.post("/v1/classify", response_model=ClassifyResponse)
def classify(
    payload: ClassifyRequest,
    request: Request,
    authorization: str | None = Header(default=None),
) -> ClassifyResponse:
    require_token(authorization)
    text_result = score_text(payload.text)
    if payload.kind == "text":
        return ClassifyResponse(sexual_score=text_result)
    if not payload.media_base64:
        raise HTTPException(status_code=status.HTTP_422_UNPROCESSABLE_ENTITY, detail="media is required")

    try:
        media = decode_media(payload.media_base64, MAX_MEDIA_BYTES)
        if payload.kind in {"image", "sticker"}:
            with detector_lock:
                detections = request.app.state.detector.detect(media)
            media_result = score_detections(detections)
        else:
            media_result = classify_video(request.app.state.detector, media)
    except ValueError as error:
        raise HTTPException(status_code=status.HTTP_422_UNPROCESSABLE_ENTITY, detail=str(error)) from error
    except (OSError, subprocess.SubprocessError, RuntimeError) as error:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="media classification failed",
        ) from error

    return ClassifyResponse(sexual_score=max(text_result, media_result))


def classify_video(detector: NudeDetector, media: bytes) -> float:
    with tempfile.TemporaryDirectory(prefix="wsp-safe-") as directory:
        workdir = Path(directory)
        video_path = workdir / "input-video"
        video_path.write_bytes(media)
        frames = extract_frames(video_path, workdir, VIDEO_SAMPLES)
        if not frames:
            raise RuntimeError("video produced no frames")
        with detector_lock:
            batches = detector.detect_batch([str(frame) for frame in frames])
        return max((score_detections(detections) for detections in batches), default=0.0)
