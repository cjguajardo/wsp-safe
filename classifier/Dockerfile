FROM python:3.11-slim-bookworm

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

RUN apt-get update \
    && apt-get install --no-install-recommends -y ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /srv/classifier

COPY requirements.txt ./
RUN pip install --requirement requirements.txt

COPY app ./app

RUN useradd --create-home --uid 10001 classifier \
    && chown -R classifier:classifier /srv/classifier

USER classifier

EXPOSE 8081

CMD ["uvicorn", "app.server:app", "--host", "0.0.0.0", "--port", "8081", "--workers", "1", "--no-access-log"]
