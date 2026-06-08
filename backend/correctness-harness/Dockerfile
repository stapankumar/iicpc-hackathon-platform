FROM golang:1.24.0 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o correctness-harness .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/correctness-harness .
ENTRYPOINT ["./correctness-harness"]