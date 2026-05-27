FROM golang:1.24.0 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o telemetry-service .

FROM alpine:3.19
RUN adduser -D -s /bin/sh appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/telemetry-service .
CMD ["./telemetry-service"]