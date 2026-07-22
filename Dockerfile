FROM golang:1.25-bookworm AS builder

RUN apt-get update \
    && apt-get install --no-install-recommends -y build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=1 \
    go build -trimpath -ldflags="-s -w" -o /out/wsp-safe ./cmd/wsp-safe

RUN CGO_ENABLED=1 \
    go build -trimpath -ldflags="-s -w" -o /out/wsp-safe-review ./cmd/wsp-safe-review

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install --no-install-recommends -y ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 wsp-safe \
    && install -d -o wsp-safe -g wsp-safe /data

COPY --from=builder /out/wsp-safe /usr/local/bin/wsp-safe
COPY --from=builder /out/wsp-safe-review /usr/local/bin/wsp-safe-review

USER wsp-safe
WORKDIR /data

ENTRYPOINT ["/usr/local/bin/wsp-safe"]
