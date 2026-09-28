ARG GO_IMAGE
ARG RUNTIME_IMAGE

FROM ${GO_IMAGE} AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY app ./app
COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux GOMAXPROCS=1 go build -p=1 -trimpath -ldflags="-s -w" -o /out/uploader ./cmd/uploader

FROM ${RUNTIME_IMAGE}

ARG ENV_MEDIA_DIR
ARG ENV_TMP_DIR
ARG ENV_PORT

RUN apk add --no-cache ca-certificates \
    && addgroup -S uploader \
    && adduser -S -G uploader uploader \
    && mkdir -p "${ENV_TMP_DIR}" "${ENV_MEDIA_DIR}" \
    && chown -R uploader:uploader "${ENV_TMP_DIR}" "${ENV_MEDIA_DIR}"

USER uploader
WORKDIR /app

COPY --from=builder /out/uploader /app/uploader

EXPOSE ${ENV_PORT}

ENTRYPOINT ["/app/uploader"]
