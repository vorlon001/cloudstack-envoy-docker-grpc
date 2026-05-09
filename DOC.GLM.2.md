# gRPC Test Project - Part 2: OpenAPI Gateway with TLS

Для генерации OpenAPI (Swagger) спецификации и готового HTTPS-сервера напрямую из `.proto` файлов в экосистеме gRPC используется стандарт от Google — **grpc-gateway**.

Этот инструмент читает специальные аннотации в proto-файле, генерирует код HTTP-роутера и файл `openapi.yaml`.

> **Важное предупреждение**: В отличие от ручного прокси из предыдущего шага, `grpc-gateway` строго следует структуре Protobuf. Это значит, что эндпоинт `getMessage` в JSON будет возвращать объект `{"messages": ["text"]}`, а не голый массив `["text"]`. Если вам нужен именно голый массив — вам нужен ручной прокси из прошлого ответа. Если вам нужен стандартный OpenAPI и автогенерация — читайте ниже.

### Шаг 1. Обновляем `proto/chat.proto`

Нам нужно добавить HTTP-маршруты (путем импорта `google/api/annotations.proto`) и настройки для генерации OpenAPI.

```protobuf
syntax = "proto3";

package chat;

option go_package = "./proto";

// Импортируем аннотации для HTTP маршрутов
import "google/api/annotations.proto";
// Импортируем настройки для генерации OpenAPI
import "protoc-gen-openapiv2/options/annotations.proto";

// Глобальные настройки для OpenAPI файла
option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_swagger) = {
  info: {
    title: "Chat gRPC-Gateway API";
    version: "1.0";
    description: "API сгенерировано из protobuf";
  };
  schemes: HTTP;
  schemes: HTTPS;
  host: "localhost:8081";
  base_path: "";
};

message SendMessageRequest {
  string username = 1;
  string message = 2;
}

message SendMessageResponse {
  bool success = 1;
}

message GetMessageRequest {
  string username = 1;
}

message GetMessageResponse {
  repeated string messages = 1;
}

message ApiVersionRequest {}

message ApiVersionResponse {
  string version = 1;
}

service ChatService {
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse) {
    // Привязываем к POST /v1/messages
    option (google.api.http) = {
      post: "/v1/messages"
      body: "*"
    };
  }

  rpc GetMessage(GetMessageRequest) returns (GetMessageResponse) {
    // Привязываем к GET /v1/messages/alice
    option (google.api.http) = {
      get: "/v1/messages/{username}"
    };
  }

  rpc ApiVersion(ApiVersionRequest) returns (ApiVersionResponse) {
    // Привязываем к GET /v1/version
    option (google.api.http) = {
      get: "/v1/version"
    };
  }
}
```

### Шаг 2. Создаем сервер OpenAPI с TLS (`cmd/openapi-server/main.go`)

Этот сервер использует сгенерированный grpc-gateway код для проксирования HTTPS запросов в gRPC с TLS, а также отдает сгенерированный `openapi.yaml`.

```go
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "grpc-app/proto"
)

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load CA certificate
	caCert, err := ioutil.ReadFile("certs/ca.crt")
	if err != nil {
		return nil, err
	}

	// Create certificate pool and add CA certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, err
	}

	// Create TLS credentials
	config := &tls.Config{
		RootCAs:            certPool,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	return credentials.NewTLS(config), nil
}

func main() {
	ctx := context.Background()

	// 1. Создаем СТАНДАРТНЫЙ http мультиплексор для статики (openapi.yaml)
	rootMux := http.NewServeMux()

	// 2. Создаем мультиплексор grpc-gateway v2 для API
	grpcGatewayMux := runtime.NewServeMux()

	// Подключаемся к gRPC серверу с TLS
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	err = pb.RegisterChatServiceHandlerFromEndpoint(ctx, grpcGatewayMux, "server:50051", opts)
	if err != nil {
		log.Fatalf("Failed to register gateway: %v", err)
	}

	// 3. Привязываем gRPC шлюз к корню
	rootMux.Handle("/", grpcGatewayMux)

	// 4. Привязываем отдачу файла openapi.yaml
	rootMux.Handle("/openapi.yaml", http.FileServer(http.Dir("./")))

	// Load HTTPS certificate
	cert, err := tls.LoadX509KeyPair("certs/https.crt", "certs/https.key")
	if err != nil {
		log.Fatalf("Failed to load HTTPS certificate: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:      "0.0.0.0:8081",
		Handler:   rootMux,
		TLSConfig: tlsConfig,
	}

	log.Println("OpenAPI Gateway Server (v2) with TLS is running on :8081")
	log.Println("Spec URL: https://localhost:8081/openapi.yaml")
	log.Println("Swagger UI: https://petstore.swagger.io/?url=https://localhost:8081/openapi.yaml")

	if err := server.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
}
```

### Шаг 3. Генерация кода с OpenAPI

Для генерации кода с поддержкой OpenAPI нужно установить дополнительные плагины:

```bash
# Установка плагинов для OpenAPI
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0

# Скачивание сторонних proto файлов
mkdir -p third_party/google/protobuf third_party/google/api third_party/protoc-gen-openapiv2/options
curl -sSL https://raw.githubusercontent.com/protocolbuffers/protobuf/refs/heads/main/src/google/protobuf/descriptor.proto -o third_party/google/protobuf/descriptor.proto
curl -sSL https://raw.githubusercontent.com/protocolbuffers/protobuf/refs/heads/main/src/google/protobuf/struct.proto -o third_party/google/protobuf/struct.proto
curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto -o third_party/google/api/annotations.proto
curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto -o third_party/google/api/http.proto
curl -sSL https://raw.githubusercontent.com/grpc-ecosystem/grpc-gateway/main/protoc-gen-openapiv2/options/annotations.proto -o third_party/protoc-gen-openapiv2/options/annotations.proto
curl -sSL https://raw.githubusercontent.com/grpc-ecosystem/grpc-gateway/main/protoc-gen-openapiv2/options/openapiv2.proto -o third_party/protoc-gen-openapiv2/options/openapiv2.proto

# Генерация кода
protoc -I proto -I third_party \
	--go_out=. \
	--go-grpc_out=. \
	--grpc-gateway_out=. \
	--openapiv2_out=. \
	--openapiv2_opt=allow_merge=true,merge_file_name=openapi,output_format=yaml \
	proto/chat.proto

mv openapi.swagger.yaml openapi.yaml
```

### Шаг 4. Обновляем `docker-compose.yml`

Добавьте сервис `openapi` в ваш `docker-compose.yml`:

```yaml
networks:
  app-network:
    driver: bridge

services:
  # ... (ваши сервисы server, proxy, client остаются без изменений) ...

  # OpenAPI (grpc-gateway v2) с TLS
  openapi:
    build:
      context: .
      args:
        APP_NAME: openapi-server
    ports:
      - "18081:8081"
    environment:
      - GRPC_SERVER_URL=server:50051
    volumes:
      - ./certs:/app/certs:ro
    depends_on:
      - server
    networks:
      - app-network
```

### Шаг 5. Тестирование

Запустите сервисы:
```bash
docker-compose up --build
```

Тестирование OpenAPI Gateway с TLS:

```bash
# Получить OpenAPI спецификацию
curl -k https://localhost:18081/openapi.yaml

# Отправить сообщение
curl -k -X POST https://localhost:18081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"username": "bob", "message": "Hello from OpenAPI!"}'

# Получить сообщения
curl -k https://localhost:18081/v1/messages/bob

# Получить версию API
curl -k https://localhost:18081/v1/version
```

### Резюме

В этом руководстве мы добавили:
1. **OpenAPI Gateway с TLS** - автоматически генерирует REST API из proto-аннотаций
2. **HTTPS сервер** - использует сертификат `https.crt` для защищенного соединения
3. **TLS подключение к gRPC** - использует CA сертификат для валидации сервера

OpenAPI спецификация доступна по адресу `https://localhost:18081/openapi.yaml` и может быть использована в Swagger UI.
