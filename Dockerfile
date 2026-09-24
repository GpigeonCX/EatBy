FROM node:24-alpine AS web
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
COPY cmd/server/web/ /src/cmd/server/web/
RUN npm run build

FROM golang:1.24-alpine AS backend
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/cmd/server/web/ cmd/server/web/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /pantry ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates su-exec tzdata && addgroup -S pantry && adduser -S pantry -G pantry && mkdir -p /data && chown pantry:pantry /data
COPY --from=backend /pantry /usr/local/bin/pantry
COPY deploy/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
