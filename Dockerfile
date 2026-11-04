FROM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /cliamp-server

# Node.js is used by yt-dlp for YouTube JavaScript challenge handling.
# The runtime also includes FFmpeg and Python yt-dlp with EJS components.
FROM node:24-bookworm-slim
RUN apt-get update && \
    apt-get install -y --no-install-recommends ffmpeg python3 python3-pip ca-certificates && \
    python3 -m pip install --break-system-packages --no-cache-dir 'yt-dlp[default]' && \
    mkdir -p /config /music /data && \
    chown -R 65532:65532 /data /music && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /cliamp-server /usr/bin/cliamp-server
ENV XDG_CONFIG_HOME=/config \
    CLIAMP_STATIONS_FILE=/data/stations.json \
    CLIAMP_BAN_FILE=/data/banned-ips.json
USER 65532:65532
EXPOSE 8000
ENTRYPOINT ["/usr/bin/cliamp-server"]
