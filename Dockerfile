# Build stage: compile a static binary (no C libraries needed).
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mpp .

# Runtime stage: a small image that runs as a non-root user.
FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app app && mkdir /data && chown app:app /data
WORKDIR /app
COPY --from=build /out/mpp /app/mpp
COPY data /app/data
ENV MPP_DATA_DIR=/app/data \
    MPP_CHARACTERS_FILE=/data/characters.json \
    MPP_ADDR=:8080
USER app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/mpp"]
CMD ["serve"]
