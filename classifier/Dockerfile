FROM python:3.11-slim-bookworm

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    OPENNSFW2_HOME=/srv/classifier/models \
    PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

RUN apt-get update \
    && apt-get install --no-install-recommends -y ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /srv/classifier

COPY requirements.txt ./
RUN pip install --requirement requirements.txt

# Instala solamente Chromium y sus bibliotecas para limitar el tamaño de la imagen.
RUN playwright install --with-deps chromium

# Descarga los pesos durante la construcción para no depender de Internet al iniciar.
RUN python -c "import opennsfw2 as n; n.make_open_nsfw_model()"

COPY app ./app

RUN useradd --create-home --uid 10001 classifier \
    && chown -R classifier:classifier /srv/classifier /ms-playwright

USER classifier

EXPOSE 8081

CMD ["uvicorn", "app.server:app", "--host", "0.0.0.0", "--port", "8081", "--workers", "1", "--no-access-log"]
