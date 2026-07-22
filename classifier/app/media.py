import base64
import binascii
import subprocess
from pathlib import Path

from app.moderation import frame_interval


def decode_media(encoded: str, max_bytes: int) -> bytes:
    if max_bytes < 1:
        raise ValueError("max_bytes must be positive")
    if len(encoded) > ((max_bytes + 2) // 3) * 4 + 4:
        raise ValueError("media exceeds configured limit")
    try:
        decoded = base64.b64decode(encoded, validate=True)
    except (binascii.Error, ValueError) as error:
        raise ValueError("media_base64 is invalid") from error
    if len(decoded) > max_bytes:
        raise ValueError("media exceeds configured limit")
    return decoded


def probe_duration(video_path: Path) -> float:
    result = subprocess.run(
        [
            "ffprobe",
            "-v",
            "error",
            "-show_entries",
            "format=duration",
            "-of",
            "default=noprint_wrappers=1:nokey=1",
            str(video_path),
        ],
        check=True,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return float(result.stdout.strip())


def extract_frames(video_path: Path, output_dir: Path, samples: int) -> list[Path]:
    try:
        duration = probe_duration(video_path)
    except (OSError, ValueError, subprocess.SubprocessError):
        duration = 0.0
    interval = frame_interval(duration, samples)
    output_pattern = output_dir / "frame-%03d.jpg"
    subprocess.run(
        [
            "ffmpeg",
            "-nostdin",
            "-v",
            "error",
            "-i",
            str(video_path),
            "-vf",
            f"fps=1/{interval:.6f}",
            "-frames:v",
            str(samples),
            "-q:v",
            "3",
            str(output_pattern),
        ],
        check=True,
        capture_output=True,
        timeout=90,
    )
    return sorted(output_dir.glob("frame-*.jpg"))
