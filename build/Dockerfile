ARG GO_IMAGE=golang:1.26.6-alpine
ARG RUNTIME_IMAGE=alpine:3.23

FROM ${GO_IMAGE} AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY app ./app
COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux GOMAXPROCS=1 go build -p=1 -trimpath -ldflags="-s -w" -o /out/pal-media-server ./cmd/server

FROM ${RUNTIME_IMAGE}

ARG ENV_MEDIA_DIR
ARG ENV_TMP_DIR
ARG ENV_PORT

RUN apk add --no-cache ca-certificates ffmpeg tzdata \
    && addgroup -S uploader \
    && adduser -S -G uploader uploader \
    && mkdir -p "${ENV_TMP_DIR}" "${ENV_MEDIA_DIR}" \
    && chown -R uploader:uploader "${ENV_TMP_DIR}" "${ENV_MEDIA_DIR}"

USER uploader
WORKDIR /app

COPY --from=builder /out/pal-media-server /app/pal-media-server

EXPOSE ${ENV_PORT}

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${ENV_PORT}/readyz" || exit 1

ENTRYPOINT ["/app/pal-media-server"]
