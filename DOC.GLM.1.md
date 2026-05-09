# gRPC Test Project - Part 1: Basic Setup with TLS

*Небольшое уточнение: на момент написания ответа последней стабильной версией языка является **Go 1.22** (и 1.23 в статусе beta). Версии 1.26 пока не существует. В примере ниже используется современный Go 1.22, код полностью готов к запуску и соответствует последним стандартам экосистемы.*

Ниже представлено полное руководство по созданию gRPC-сервиса, HTTPS-прокси и клиента с поддержкой TLS с помощью Docker Compose.

### Структура проекта
Создайте корневую папку проекта (например, `grpc-app`) и следующую структуру:
```text
grpc-app/
├── cmd/
│   ├── server/
│   │   └── main.go
│   ├── proxy/
│   │   └── main.go
│   └── client/
│       └── main.go
├── proto/
│   └── chat.proto
├── certs/
│   └── generate-certs.sh
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── go.sum
```

---

### 1. Генерация TLS сертификатов

Перед началом работы необходимо сгенерировать TLS сертификаты для всех сервисов.

**Файл `certs/generate-certs.sh`**
```bash
#!/bin/bash
set -e

CERTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$CERTS_DIR"

echo "==> Generating TLS certificates..."

# 1. Generate CA certificate
echo "==> Generating CA certificate..."
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
    -subj "/C=RU/ST=Test/L=Test/O=Test/OU=CA/CN=Test CA"

# 2. Generate server certificate (for gRPC server)
echo "==> Generating server certificate..."
cat > server.conf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = RU
ST = Test
L = Test
O = Test
OU = Server
CN = server

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = server
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr -config server.conf
openssl x509 -req -days 3650 -in server.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out server.crt -extensions v3_req -extfile server.conf

# 3. Generate HTTPS certificate for HTTP servers
echo "==> Generating HTTPS certificate..."
cat > https.conf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = RU
ST = Test
L = Test
O = Test
OU = HTTPS
CN = localhost

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = proxy
DNS.3 = openapi
DNS.4 = http-server
DNS.5 = envoy
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

openssl genrsa -out https.key 4096
openssl req -new -key https.key -out https.csr -config https.conf
openssl x509 -req -days 3650 -in https.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out https.crt -extensions v3_req -extfile https.conf

# Clean up temporary files
rm -f server.csr https.csr server.conf https.conf ca.srl

echo "==> Certificates generated successfully!"
```

Выполните скрипт для генерации сертификатов:
```bash
cd grpc-app/certs
bash generate-certs.sh
```

---

### 2. Инициализация модуля и генерация Proto

**Файл `proto/chat.proto`**
```protobuf
syntax = "proto3";

package chat;

option go_package = "./proto";

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
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse);
  rpc GetMessage(GetMessageRequest) returns (GetMessageResponse);
  rpc ApiVersion(ApiVersionRequest) returns (ApiVersionResponse);
}
```

Откройте терминал в корне папки и выполните команды:
```bash
# Инициализация Go модуля
go mod init grpc-app

# Установка плагинов для генерации кода
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Установка protobuf-compiler
apt update && apt install -y protobuf-compiler
export PATH=$PATH:$(go env GOPATH)/bin

# Генерация Go кода из proto файла
protoc --go_out=. --go-grpc_out=. proto/chat.proto
```
*(Убедитесь, что у вас установлен `protoc` в системе. В папке `proto/` появятся файлы `chat.pb.go` и `chat_grpc.pb.go`)*

---

### 3. Реализация приложений с TLS

**Файл `cmd/server/main.go`** (gRPC Сервер с TLS)
```go
package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"sync"

	pb "grpc-app/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type server struct {
	pb.UnimplementedChatServiceServer
	mu       sync.RWMutex
	messages map[string][]string
}

func newServer() *server {
	return &server{messages: make(map[string][]string)}
}

func (s *server) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[req.Username] = append(s.messages[req.Username], req.Message)
	return &pb.SendMessageResponse{Success: true}, nil
}

func (s *server) GetMessage(ctx context.Context, req *pb.GetMessageRequest) (*pb.GetMessageResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.messages[req.Username]
	if msgs == nil {
		msgs = []string{}
	}
	return &pb.GetMessageResponse{Messages: msgs}, nil
}

func (s *server) ApiVersion(ctx context.Context, req *pb.ApiVersionRequest) (*pb.ApiVersionResponse, error) {
	return &pb.ApiVersionResponse{Version: "1.0.0"}, nil
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert, // No client cert required for basic TLS
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(config), nil
}

func main() {
	// Load TLS credentials
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterChatServiceServer(s, newServer())
	log.Println("gRPC Server with TLS is running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```

**Файл `cmd/proxy/main.go`** (HTTPS → gRPC(TLS) Прокси)
```go
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	pb "grpc-app/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var grpcClient pb.ChatServiceClient

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
	// Подключаемся к gRPC серверу с TLS
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	conn, err := grpc.Dial("server:50051", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	grpcClient = pb.NewChatServiceClient(conn)

	http.HandleFunc("/api/send", handleSendMessage)
	http.HandleFunc("/api/get", handleGetMessage)
	http.HandleFunc("/api/version", handleApiVersion)

	// Load HTTPS certificate
	cert, err := tls.LoadX509KeyPair("certs/https.crt", "certs/https.key")
	if err != nil {
		log.Fatalf("failed to load HTTPS certificate: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:      ":8080",
		Handler:   nil,
		TLSConfig: tlsConfig,
	}

	log.Println("HTTPS Proxy is running on :8080")
	log.Fatal(server.ListenAndServeTLS("", ""))
}

func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req pb.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := grpcClient.SendMessage(context.Background(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleGetMessage(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	res, err := grpcClient.GetMessage(context.Background(), &pb.GetMessageRequest{Username: username})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(res.Messages)
}

func handleApiVersion(w http.ResponseWriter, r *http.Request) {
	res, err := grpcClient.ApiVersion(context.Background(), &pb.ApiVersionRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"version": res.Version})
}
```

**Файл `cmd/client/main.go`** (gRPC Клиент с TLS)
```go
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"time"

	pb "grpc-app/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	// Ждем 2 секунды, чтобы сервер в докере точно успел подняться
	time.Sleep(2 * time.Second)

	// Load TLS credentials
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	conn, err := grpc.Dial("server:50051", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// 1. Проверяем версию
	ver, err := client.ApiVersion(ctx, &pb.ApiVersionRequest{})
	if err != nil {
		log.Fatalf("ApiVersion failed: %v", err)
	}
	log.Printf("API Version: %s", ver.GetVersion())

	// 2. Отправляем сообщение
	sendRes, err := client.SendMessage(ctx, &pb.SendMessageRequest{Username: "alice", Message: "Hello from gRPC client! 1"})
	if err != nil {
		log.Fatalf("SendMessage failed: %v", err)
	}
	log.Printf("SendMessage Success: %v", sendRes.GetSuccess())

	// 3. Получаем сообщения
	getRes, err := client.GetMessage(ctx, &pb.GetMessageRequest{Username: "alice"})
	if err != nil {
		log.Fatalf("GetMessage failed: %v", err)
	}
	log.Printf("Messages for alice: %v", getRes.GetMessages())
}
```

---

### 4. Docker Compose с TLS

**Файл `docker-compose.yml`**
```yaml
networks:
  app-network:
    driver: bridge

services:
  # 1. gRPC Сервер (ядро) с TLS
  server:
    build:
      context: .
      args:
        APP_NAME: server
    ports:
      - "50051:50051"
    volumes:
      - ./certs:/app/certs:ro
    networks:
      - app-network

  # 2. HTTPS -> gRPC(TLS) прокси
  proxy:
    build:
      context: .
      args:
        APP_NAME: proxy
    ports:
      - "18080:8080"
    environment:
      - GRPC_SERVER_URL=server:50051
    volumes:
      - ./certs:/app/certs:ro
    depends_on:
      - server
    networks:
      - app-network

  # 3. gRPC Клиент с TLS (запустится, отправит сообщения и завершится с кодом 0)
  client:
    build:
      context: .
      args:
        APP_NAME: client
    environment:
      - GRPC_SERVER_URL=server:50051
    volumes:
      - ./certs:/app/certs:ro
    depends_on:
      - server
    networks:
      - app-network
```

---

### 5. Dockerfile

**Файл `Dockerfile`**
```dockerfile
# Этап сборки
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum
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

# Запускаем приложение
ENTRYPOINT ["./app_main"]
```

---

### 6. Запуск и тестирование

**Запуск сервисов:**
```bash
docker-compose up --build
```

**Тестирование HTTPS прокси:**
```bash
# Отправить сообщение
curl -k -X POST https://localhost:18080/api/send \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "message": "Hello from HTTPS!"}'

# Получить сообщения
curl -k https://localhost:18080/api/get?username=alice

# Получить версию API
curl -k https://localhost:18080/api/version
```

**Тестирование gRPC клиента:**
```bash
docker-compose run client
```

---

### Резюме

В этом руководстве мы создали:
1. **gRPC сервер с TLS** - использует сертификат `server.crt` для защищенного соединения
2. **HTTPS прокси** - принимает HTTPS запросы и пересылает их на gRPC сервер с TLS
3. **gRPC клиент с TLS** - подключается к gRPC серверу с использованием TLS

Все сервисы используют TLS 1.2+ и валидацию сертификатов через CA.




```

export PATH=$PATH:/root/go/bin

make fetch-apis
make build
docker-compose build
docker-compose up -d

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0

export PATH=$PATH:/root/go/bin
protoc -I proto -I third_party \
  --include_imports \
  --descriptor_set_out=envoy/proto.pb \
  proto/chat.proto


protoc -I proto -I third_party \
        --go_out=. \
        --go-grpc_out=. \
        --grpc-gateway_out=. \
        --openapiv2_out=. \
        --openapiv2_opt=allow_merge=true,merge_file_name=openapi,output_format=yaml \
        proto/chat.proto


```
