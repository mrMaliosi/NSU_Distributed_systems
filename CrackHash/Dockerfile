FROM golang:1.23.1 AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o manager-exe ./cmd/manager/main.go
RUN CGO_ENABLED=0 go build -o worker-exe ./cmd/worker/main.go


FROM alpine:latest

WORKDIR /app/

# Копируем оба бинарника в /app
COPY --from=builder /app/manager-exe /app/
COPY --from=builder /app/worker-exe /app/

RUN chmod +x /app/manager-exe /app/worker-exe

EXPOSE 57107
