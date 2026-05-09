server proxy.PHONY: all build clean docker-up docker-down help

# Переменные
BUILDER_IMAGE=golang:1.26-alpine
BIN_DIR=bin
APPS=server-https server-http proxy-https proxy-http client-http client-https openapi-server-https openapi-server-http https-server http-server

# Установка всех плагинов (включая OpenAPI)
INSTALL_ALL_PLUGINS=go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
                    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
                    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0 && \
                    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0

# Макрос для сборки конкретного приложения
# 1. Монтирует текущую папку в /app
# 2. Монтирует папку ./bin для сохранения результата
# 3. Монтирует кэши Go модулей и сборки (чтобы не качать зависимости каждый раз)
define build_app
    docker run --rm \
        -v $(PWD):/app \
        -v gomodcache:/go/pkg/mod \
        -v gobuildcache:/root/.cache/go-build \
        -v $(PWD)/$(BIN_DIR):/app/$(BIN_DIR) \
        -w /app \
        $(BUILDER_IMAGE) \
        sh -c "set -x && apk add --no-cache git && \
               go mod tidy && \
               CGO_ENABLED=0 go build -o $(BIN_DIR)/$(1) ./cmd/$(1) && \
               chmod +x $(BIN_DIR)/$(1)"
endef

# 1. Скачивание зависимостей для OpenAPI (выполняется один раз)
fetch-apis:
	@echo "==> Скачивание сторонних proto файлов..."
	@mkdir -p third_party/google/protobuf third_party/google/api third_party/protoc-gen-openapiv2/options
	@curl -sSL https://raw.githubusercontent.com/protocolbuffers/protobuf/refs/heads/main/src/google/protobuf/descriptor.proto -o third_party/google/protobuf/descriptor.proto
	@curl -sSL https://raw.githubusercontent.com/protocolbuffers/protobuf/refs/heads/main/src/google/protobuf/struct.proto -o third_party/google/protobuf/struct.proto
	@curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto -o third_party/google/api/annotations.proto
	@curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto -o third_party/google/api/http.proto
	@curl -sSL https://raw.githubusercontent.com/grpc-ecosystem/grpc-gateway/main/protoc-gen-openapiv2/options/annotations.proto -o third_party/protoc-gen-openapiv2/options/annotations.proto
	@curl -sSL https://raw.githubusercontent.com/grpc-ecosystem/grpc-gateway/main/protoc-gen-openapiv2/options/openapiv2.proto -o third_party/protoc-gen-openapiv2/options/openapiv2.proto
	@echo "==> Скачивание завершено"
	$(INSTALL_ALL_PLUGINS)
	protoc -I proto -I third_party \
		--go_out=. \
		--go-grpc_out=. \
		--grpc-gateway_out=. \
		--openapiv2_out=. \
                --openapiv2_opt=allow_merge=true,merge_file_name=openapi,output_format=yaml \
		proto/chat.proto
	protoc -I proto -I third_party \
                --include_imports \
                --descriptor_set_out=envoy/proto.pb \
                proto/chat.proto
	mv openapi.swagger.yaml openapi.yaml

# --- Цели (Targets) ---

all: build

build: $(APPS)

# Динамическое создание целей для каждого приложения (make server, make proxy, make client-http client-https)
server-https server-http proxy-https proxy-http client-http client-https openapi-server-https openapi-server-http https-server http-server:
	@mkdir -p $(BIN_DIR)
	@echo "==> Сборка бинарника: $@"
	@$(call build_app,$@)
	@echo "==> Готово: $(BIN_DIR)/$@"

clean:
	@echo "==> Очистка папки $(BIN_DIR)..."
	@rm -rf $(BIN_DIR)
	#docker volume rm gomodcache gobuildcache
	@echo "==> Очистка завершена"

# Цели для работы с docker-compose (если нужно запустить всё как сервисы)
docker-up:
	@echo "==> Запуск сервисов в Docker..."
	docker-compose up --build -d

docker-down:
	@echo "==> Остановка сервисов..."
	docker-compose down

help:
	@echo "Доступные команды:"
	@echo "  make build        - Собрать все бинарники (server, proxy, client) в папку ./bin"
	@echo "  make clean        - Удалить папку ./bin с бинарниками"
	@echo "  make docker-up    - Поднять инфраструктуру через docker-compose"
	@echo "  make docker-down  - Остановить инфраструктуру"
