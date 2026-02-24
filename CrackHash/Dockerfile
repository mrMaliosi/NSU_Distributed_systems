FROM golang:1.23.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o manager ./cmd/Manager

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/manager /manager

WORKDIR /

EXPOSE 57107

ENTRYPOINT ["./manager"]