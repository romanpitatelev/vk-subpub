FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY ../../go.mod ../../go.sum ./
RUN go mod download

COPY ../.. .
RUN go build -o subscription-service ./cmd/subscription-service/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/subscription-service .

EXPOSE 9000
CMD ["./subscription-service"]
