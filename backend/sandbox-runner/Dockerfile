FROM golang:1.24.0 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o sandbox-runner .

FROM alpine:3.19
RUN adduser -D -s /bin/sh appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/sandbox-runner .
CMD ["./sandbox-runner"]