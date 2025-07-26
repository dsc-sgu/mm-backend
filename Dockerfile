FROM golang:1.24-alpine as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o ./webserver ./cmd/webserver/main.go
RUN go build -o ./runsql ./cmd/runsql/main.go

FROM alpine:3.20
COPY --from=builder /app/webserver /app/webserver
COPY --from=builder /app/runsql /app/runsql
COPY --from=builder /app/db/CreateTables.sql /app/db/CreateTables.sql
COPY --from=builder /app/db/DropTables.sql /app/db/DropTables.sql
