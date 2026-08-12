FROM golang:1.24-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/netcontrol-worker ./cmd/worker
RUN mkdir -p /out/firmwares

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/netcontrol-worker /netcontrol-worker
COPY --from=build --chown=65532:65532 /out/firmwares /var/lib/netcontrol/firmwares
ENTRYPOINT ["/netcontrol-worker"]
