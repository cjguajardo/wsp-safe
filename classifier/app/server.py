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

from app.logging import (
    log_classification_result,
    log_http_request,
    log_requests_enabled,
    request_duration_ms,
    request_started_at,
)
from app.media import decode_media, extract_frames
from app.moderation import score_detections, score_text
from app.providers import GoogleSafeSearch, OpenNSFW2Classifier, maximum_score
from app.sites import (
    CloudflareRadarProvider,
    DomainCategoryStore,
    IPQualityScoreProvider,
    PlaywrightScreenshotProvider,
    RotatingSiteGateway,
    classify_text_sites,
)


MAX_MEDIA_BYTES = int(os.getenv("CLASSIFIER_MAX_MEDIA_BYTES", str(20 << 20)))
VIDEO_SAMPLES = int(os.getenv("CLASSIFIER_VIDEO_SAMPLES", "6"))
CLASSIFIER_TOKEN = os.getenv("CLASSIFIER_TOKEN", "")
CLASSIFIER_LOG_REQUESTS = log_requests_enabled(
    os.getenv("CLASSIFIER_LOG_REQUESTS", "false")
)
GOOGLE_SAFESEARCH_API_KEY = os.getenv("GOOGLE_SAFESEARCH_API_KEY", "")
IPQS_API_KEY = os.getenv("IPQS_API_KEY", "")
CLOUDFLARE_ACCOUNT_ID = os.getenv("CLOUDFLARE_ACCOUNT_ID", "")
CLOUDFLARE_API_TOKEN = os.getenv("CLOUDFLARE_API_TOKEN", "")
SITE_DB_PATH = os.getenv("SITE_DB_PATH", "/data/site-categories.db")
SITE_PLAYWRIGHT_ENABLED = log_requests_enabled(
    os.getenv("SITE_PLAYWRIGHT_ENABLED", "true")
)
SITE_NSFW_THRESHOLD = float(os.getenv("SITE_NSFW_THRESHOLD", "0.25"))
SITE_MAX_URLS_PER_MESSAGE = int(os.getenv("SITE_MAX_URLS_PER_MESSAGE", "3"))
SITE_REQUEST_TIMEOUT = float(os.getenv("SITE_REQUEST_TIMEOUT", "30"))
detector_lock = threading.Lock()


class ClassifyRequest(BaseModel):
    message_id: str = Field(min_length=1, max_length=256)
    sender_id: str = Field(default="", max_length=256)
    kind: Literal["text", "image", "video", "sticker"]
    mime_type: str = Field(default="", max_length=128)
    text: str = Field(default="", max_length=100_000)
    media_base64: str = ""


class SiteClassificationRequest(BaseModel):
    url: str = Field(min_length=1, max_length=4096)


class SiteClassificationResponse(BaseModel):
    domain: str
    categories: list[str]
    nsfw: bool
    score: float
    provider: str
    cached: bool


class ClassifyResponse(BaseModel):
    sexual_score: float
    sexual_minors_score: float = 0.0
    uncertain: bool = False
    sites: list[SiteClassificationResponse] = Field(default_factory=list)


@asynccontextmanager
async def lifespan(app: FastAPI):
    if SITE_MAX_URLS_PER_MESSAGE < 1:
        raise RuntimeError("SITE_MAX_URLS_PER_MESSAGE debe ser positivo")
    if SITE_REQUEST_TIMEOUT <= 0:
        raise RuntimeError("SITE_REQUEST_TIMEOUT debe ser positivo")
    app.state.detector = NudeDetector()
    app.state.opennsfw2 = OpenNSFW2Classifier()
    app.state.safe_search = GoogleSafeSearch(GOOGLE_SAFESEARCH_API_KEY)
    providers = []
    if IPQS_API_KEY.strip():
        providers.append(IPQualityScoreProvider(IPQS_API_KEY, SITE_REQUEST_TIMEOUT))
    if CLOUDFLARE_ACCOUNT_ID.strip() and CLOUDFLARE_API_TOKEN.strip():
        providers.append(CloudflareRadarProvider(
            CLOUDFLARE_ACCOUNT_ID,
            CLOUDFLARE_API_TOKEN,
            SITE_REQUEST_TIMEOUT,
        ))
    if SITE_PLAYWRIGHT_ENABLED:
        providers.append(PlaywrightScreenshotProvider(
            lambda screenshot: score_screenshot(app, screenshot),
            SITE_NSFW_THRESHOLD,
            SITE_REQUEST_TIMEOUT,
        ))
    if not providers:
        raise RuntimeError("no hay verificadores de sitios habilitados")
    app.state.site_gateway = RotatingSiteGateway(
        DomainCategoryStore(SITE_DB_PATH),
        providers,
    )
    try:
        yield
    finally:
        app.state.site_gateway.close()


app = FastAPI(
    title="wsp-safe NudeNet classifier",
    docs_url=None,
    redoc_url=None,
    openapi_url=None,
    lifespan=lifespan,
)


@app.middleware("http")
async def log_request(request: Request, call_next):
    started_at = request_started_at()
    response = await call_next(request)
    if CLASSIFIER_LOG_REQUESTS and request.url.path in {"/v1/classify", "/v1/classify-site"}:
        log_http_request(request, response, request_duration_ms(started_at))
    return response


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
    started_at = request_started_at()
    require_token(authorization)
    site_analysis = classify_text_sites(
        payload.text,
        request.app.state.site_gateway,
        SITE_MAX_URLS_PER_MESSAGE,
    )
    sites = [site_response(result) for result in site_analysis.results]
    text_result = max(score_text(payload.text), site_analysis.score)
    if payload.kind == "text":
        response = ClassifyResponse(
            sexual_score=text_result,
            uncertain=site_analysis.uncertain,
            sites=sites,
        )
        log_result_if_enabled(payload, response, request_duration_ms(started_at))
        return response
    if not payload.media_base64:
        raise HTTPException(status_code=status.HTTP_422_UNPROCESSABLE_ENTITY, detail="media is required")

    try:
        media = decode_media(payload.media_base64, MAX_MEDIA_BYTES)
        if payload.kind in {"image", "sticker"}:
            with detector_lock:
                detections = request.app.state.detector.detect(media)
            media_result = maximum_score(
                score_detections(detections),
                request.app.state.opennsfw2.score(media),
                request.app.state.safe_search.score(media),
            )
        else:
            media_result = classify_video(
                request.app.state.detector,
                request.app.state.opennsfw2,
                request.app.state.safe_search,
                media,
            )
    except ValueError as error:
        raise HTTPException(status_code=status.HTTP_422_UNPROCESSABLE_ENTITY, detail=str(error)) from error
    except (OSError, subprocess.SubprocessError, RuntimeError) as error:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="media classification failed",
        ) from error

    response = ClassifyResponse(
        sexual_score=max(text_result, media_result),
        uncertain=site_analysis.uncertain,
        sites=sites,
    )
    log_result_if_enabled(payload, response, request_duration_ms(started_at))
    return response


@app.post("/v1/classify-site", response_model=SiteClassificationResponse)
def classify_site(
    payload: SiteClassificationRequest,
    request: Request,
    authorization: str | None = Header(default=None),
) -> SiteClassificationResponse:
    require_token(authorization)
    try:
        result = request.app.state.site_gateway.classify(payload.url)
    except (OSError, RuntimeError, ValueError) as error:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="site classification failed",
        ) from error
    return site_response(result)


def site_response(result) -> SiteClassificationResponse:
    return SiteClassificationResponse(
        domain=result.domain,
        categories=list(result.categories),
        nsfw=result.nsfw,
        score=result.score,
        provider=result.provider,
        cached=result.cached,
    )


def score_screenshot(app: FastAPI, media: bytes) -> float:
    with detector_lock:
        detections = app.state.detector.detect(media)
    return maximum_score(
        score_detections(detections),
        app.state.opennsfw2.score(media),
        app.state.safe_search.score(media),
    )


def log_result_if_enabled(
    payload: ClassifyRequest,
    response: ClassifyResponse,
    duration_ms: float,
) -> None:
    if not CLASSIFIER_LOG_REQUESTS:
        return
    log_classification_result(
        message_id=payload.message_id,
        sender_id=payload.sender_id,
        kind=payload.kind,
        mime_type=payload.mime_type,
        text_length=len(payload.text),
        media_base64_length=len(payload.media_base64),
        sexual_score=response.sexual_score,
        sexual_minors_score=response.sexual_minors_score,
        uncertain=response.uncertain,
        duration_ms=duration_ms,
    )


def classify_video(
    detector: NudeDetector,
    opennsfw2: OpenNSFW2Classifier,
    safe_search: GoogleSafeSearch,
    media: bytes,
) -> float:
    with tempfile.TemporaryDirectory(prefix="wsp-safe-") as directory:
        workdir = Path(directory)
        video_path = workdir / "input-video"
        video_path.write_bytes(media)
        frames = extract_frames(video_path, workdir, VIDEO_SAMPLES)
        if not frames:
            raise RuntimeError("video produced no frames")
        frame_contents = [frame.read_bytes() for frame in frames]
        with detector_lock:
            batches = detector.detect_batch([str(frame) for frame in frames])
        nudenet_score = max(
            (score_detections(detections) for detections in batches),
            default=0.0,
        )
        safe_search_score = maximum_score(
            *(safe_search.score(frame) for frame in frame_contents)
        ) if safe_search.enabled else 0.0
        return maximum_score(
            nudenet_score,
            opennsfw2.score_many(frame_contents),
            safe_search_score,
        )
