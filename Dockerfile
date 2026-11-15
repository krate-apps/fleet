# Fleet host image (linux/amd64). Clients (rTorrent / qBittorrent / Deluge /
# Transmission) stay outside this image — see docs/user/docker.md.

ARG GO_VERSION=1.22
ARG NODE_VERSION=22

# --- frontend ---
FROM node:${NODE_VERSION}-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
ARG FLEET_UI_BASE=/
ENV FLEET_UI_BASE=${FLEET_UI_BASE}
RUN npm run build

# --- go binary ---
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=0.0.0-dev
ARG COMMIT=unknown
RUN rm -rf cmd/fleet/web \
  && mkdir -p cmd/fleet/web \
  && cp -a web/dist/. cmd/fleet/web/ \
  && touch cmd/fleet/web/.gitkeep \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    make build VERSION="v${VERSION#v}" COMMIT="${COMMIT}" \
  && mkdir -p /out \
  && cp -a bin/fleet bin/fleet-mktorrent bin/fleet-dumptorrent /out/

# --- runtime ---
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget su-exec \
  && adduser -D -H -u 1000 fleet \
  && mkdir -p /config /data \
  && chown -R fleet:fleet /config /data
COPY --from=build /out/fleet /out/fleet-mktorrent /out/fleet-dumptorrent /usr/local/bin/
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
