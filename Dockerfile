FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o ./server ./cmd/server/main.go

FROM alpine:3.20
COPY --from=builder /app/server /app/server
