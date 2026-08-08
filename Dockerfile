FROM node:24-alpine AS web-builder

WORKDIR /src
COPY web/package.json ./web/package.json
RUN npm --prefix web install
COPY web ./web
COPY cmd/clist/web-dist ./cmd/clist/web-dist
RUN npm --prefix web run build

FROM golang:1.26.5-alpine AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web-builder /src/cmd/clist/web-dist ./cmd/clist/web-dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/clist ./cmd/clist

FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=go-builder /out/clist /app/clist
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/app/clist", "healthcheck"]
ENTRYPOINT ["/app/clist"]
