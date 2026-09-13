FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/backend \
    ./cmd/app

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S agripadi \
    && adduser -S -G agripadi -h /app agripadi

WORKDIR /app

COPY --from=builder /out/backend ./backend
COPY --from=builder /app/config.yml ./config.yml

RUN mkdir -p /app/uploads \
    && chown -R agripadi:agripadi /app

USER agripadi

EXPOSE 8181

CMD ["./backend"]
