# Fleet host image (linux/amd64). Clients (rTorrent / qBittorrent / Deluge /
# Transmission) stay outside this image — see docs/user/docker.md.

ARG GO_VERSION=1.26
ARG NODE_VERSION=26

# --- frontend ---
FROM node:${NODE_VERSION}-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
ARG FLEET_UI_BASE=/
ENV FLEET_UI_BASE=${FLEET_UI_BASE}
RUN npm run build

# --- official JSON torrent helpers (tomcdj71), musl to match runtime ---
FROM alpine:3.21 AS torrent-tools
RUN apk add --no-cache build-base cmake git openssl-dev linux-headers
WORKDIR /src
RUN git clone --depth 1 https://github.com/tomcdj71/mktorrent.git \
  && cmake -S mktorrent -B mktorrent/build \
    -DCMAKE_BUILD_TYPE=Release \
    -DMKTORRENT_OPENSSL=ON \
    -DMKTORRENT_PTHREADS=ON \
    -DMKTORRENT_LONG_OPTIONS=ON \
  && cmake --build mktorrent/build -j"$(nproc)" \
  && git clone --depth 1 https://github.com/tomcdj71/dumptorrent.git \
  && cmake -S dumptorrent -B dumptorrent/build -DCMAKE_BUILD_TYPE=Release \
  && cmake --build dumptorrent/build -j"$(nproc)" \
  && install -m 0755 mktorrent/build/mktorrent /usr/local/bin/mktorrent \
  && install -m 0755 dumptorrent/build/dumptorrent /usr/local/bin/dumptorrent

# --- go binary ---
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=0.0.0-dev
ARG COMMIT=unknown
# Host binary only here; torrent helpers come from torrent-tools stage.
RUN rm -rf cmd/fleet/web \
  && mkdir -p cmd/fleet/web bin \
  && cp -a web/dist/. cmd/fleet/web/ \
  && touch cmd/fleet/web/.gitkeep \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w \
      -X github.com/krate-apps/fleet/internal/version.Version=v${VERSION#v} \
      -X github.com/krate-apps/fleet/internal/version.Commit=${COMMIT}" \
      -o bin/fleet ./cmd/fleet \
  && mkdir -p /out \
  && cp -a bin/fleet /out/

# --- runtime ---
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget su-exec libstdc++ libgcc \
  && adduser -D -H -u 1000 fleet \
  && mkdir -p /config /data \
  && chown -R fleet:fleet /config /data
COPY --from=build /out/fleet /usr/local/bin/
COPY --from=torrent-tools /usr/local/bin/mktorrent /usr/local/bin/dumptorrent /usr/local/bin/
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod 755 /docker-entrypoint.sh
WORKDIR /data
EXPOSE 8080
VOLUME ["/config", "/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null || exit 1
# Start as root so bind mounts can be chown'd, then drop to fleet (see entrypoint).
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-config", "/config/config.yaml"]
