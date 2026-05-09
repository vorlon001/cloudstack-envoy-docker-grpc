# Этап сборки
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Копируем go.mod. Звездочка после go.sum (*) означает: 
# "скопируй этот файл, если он существует, но не падай с ошибкой, если его нет"
COPY go.mod go.sum* ./
RUN go mod download

# Копируем весь остальной код (включая сгенерированные proto файлы)
COPY . .

# Аргумент, который мы передаем из docker-compose
ARG APP_NAME=server

# Собираем бинарник. CGO_ENABLED=0 обязателен для Alpine
RUN CGO_ENABLED=0 GOOS=linux go build -o ./app_main ./cmd/${APP_NAME}

# Финальный этап (минимальный образ для запуска)
FROM alpine:3.23

WORKDIR /app

# Копируем собранный бинарник из этапа builder
COPY --from=builder /app/app_main .
COPY --from=builder /app/openapi.yaml ./

# Запускаем приложение
ENTRYPOINT ["./app_main"]
