FROM node:24-alpine AS web
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
COPY cmd/server/web/ /src/cmd/server/web/
RUN npm run build

FROM golang:1.24-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/cmd/server/web/ cmd/server/web/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /pantry ./cmd/server

FROM alpine:3.22
RUN addgroup -S pantry && adduser -S pantry -G pantry && mkdir -p /data && chown pantry:pantry /data
USER pantry
COPY --from=backend /pantry /usr/local/bin/pantry
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["pantry", "-data", "/data"]
