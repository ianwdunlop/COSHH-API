FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY cmd ./cmd
COPY internal/chemical ./internal/chemical
COPY internal/db ./internal/db
COPY internal/server ./internal/server
COPY internal/users ./internal/users

RUN go build -o coshh ./cmd/main.go

EXPOSE 8080

CMD ["/app/coshh"]
