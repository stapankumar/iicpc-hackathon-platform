FROM golang:1.24.0 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o submission-service .

FROM alpine:3.19
RUN adduser -D -s /bin/sh appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/submission-service .
EXPOSE 8081
CMD ["./submission-service"]