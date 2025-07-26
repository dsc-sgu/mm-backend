FROM golang:1.24-alpine

WORKDIR /app

RUN go install github.com/air-verse/air@latest
COPY .air.toml .

CMD ["air", "-c", ".air.toml"]
