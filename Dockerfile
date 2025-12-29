# Multi-stage build для минимального размера образа
FROM golang:1.25-alpine AS builder

# Устанавливаем зависимости для сборки
RUN apk add --no-cache git

# Рабочая директория
WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o highload-service .

# Финальный stage с минимальным образом
FROM alpine:latest

# Добавляем сертификаты для HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Копируем бинарник из builder
COPY --from=builder /app/highload-service .

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./highload-service"]

