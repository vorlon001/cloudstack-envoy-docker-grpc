# gRPC Test Project - Part 4: Pure HTTP Server with TLS

В этом руководстве мы создадим чистый HTTPS сервер с валидацией Protobuf, который работает независимо от gRPC.

Этот сервер:
- Принимает HTTPS запросы
- Валидирует JSON через protobuf схему
- Хранит сообщения в памяти
- Не использует gRPC под капотом

### Шаг 1. Создаем HTTPS сервер с валидацией (`cmd/http-server/main.go`)

```go
package main

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"sync"

	pb "grpc-app/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	mu       sync.RWMutex
	messages map[string][]string
}

func NewServer() *Server {
	return &Server{messages: make(map[string][]string)}
}

// responseWriterWrapper — структура, которая оборачивает стандартный ResponseWriter
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// loggingMiddleware проверяет статус после отработки хендлера
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrappedWriter := &responseWriterWrapper{ResponseWriter: w, statusCode: 200}

		// Передаем управление дальше по цепочке
		next.ServeHTTP(wrappedWriter, r)

		log.Printf("[DEBUG]  Intercepted: Path=%s Method=%s IP=%s",
			r.URL.Path, r.Method, r.RemoteAddr)
		// Если после выполнения хендлера код остался 404 — логируем
		if wrappedWriter.statusCode == http.StatusNotFound {
			log.Printf("[ERROR] 404 Intercepted: Path=%s Method=%s IP=%s",
				r.URL.Path, r.Method, r.RemoteAddr)
		}
	})
}

func main() {
	srv := NewServer()
	mux := http.NewServeMux()

	// Роуты EXACTLY как в ваших proto annotations
	mux.HandleFunc("POST /v1/messages", srv.handleSend)
	mux.HandleFunc("GET /v1/messages/{username}", srv.handleGet)
	mux.HandleFunc("GET /v1/version", srv.handleVersion)

	handlerWithLogs := loggingMiddleware(mux)

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
		Addr:      ":8082",
		Handler:   handlerWithLogs,
		TLSConfig: tlsConfig,
	}

	log.Println("Pure HTTPS + Protobuf Validation Server is running on :8082")
	log.Fatal(server.ListenAndServeTLS("", ""))
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	// 1. Читаем тело запроса в []byte
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error": "Failed to read body"}`, http.StatusBadRequest)
		return
	}

	var req pb.SendMessageRequest
	// 2. Парсим []byte через protojson
	if err := protojson.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error": "Invalid request format"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.messages[req.Username] = append(s.messages[req.Username], req.Message)
	s.mu.Unlock()

	res := &pb.SendMessageResponse{Success: true}
	writeProtoJSON(w, http.StatusCreated, res)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		http.Error(w, `{"error": "Username is required"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	msgs := s.messages[username]
	s.mu.RUnlock()

	res := &pb.GetMessageResponse{Messages: msgs}
	writeProtoJSON(w, http.StatusOK, res)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	res := &pb.ApiVersionResponse{Version: "1.0.0"}
	writeProtoJSON(w, http.StatusOK, res)
}

// Вспомогательная функция: используем правильный тип proto.Message
func writeProtoJSON(w http.ResponseWriter, status int, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// EmitDefaultValues: чтобы пустой []string возвращался как [], а не null
	opts := protojson.MarshalOptions{EmitDefaultValues: true}
	data, err := opts.Marshal(msg)
	if err != nil {
		http.Error(w, `{"error": "Internal encode error"}`, http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
```

### Шаг 2. Обновляем `docker-compose.yml`

Добавьте сервис `http-server` в ваш `docker-compose.yml`:

```yaml
networks:
  app-network:
    driver: bridge

services:
  # ... (ваши сервисы server, proxy, openapi, client остаются без изменений) ...

  # Чистый HTTPS сервер с валидацией Proto (без gRPC под капотом)
  http-server:
    build:
      context: .
      args:
        APP_NAME: http-server
    ports:
      - "18082:8082"
    volumes:
      - ./certs:/app/certs:ro
    networks:
      - app-network
```

### Шаг 3. Тестирование

Запустите сервисы:
```bash
docker-compose up --build
```

Тестирование HTTPS сервера:

```bash
# Отправить сообщение
curl -k -X POST https://localhost:18082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"username": "charlie", "message": "Hello from HTTP Server!"}'

# Получить сообщения
curl -k https://localhost:18082/v1/messages/charlie

# Получить версию API
curl -k https://localhost:18082/v1/version
```

### Резюме

В этом руководстве мы создали:
1. **Pure HTTPS сервер** - работает независимо от gRPC
2. **Protobuf валидация** - использует `protojson` для строгой валидации JSON
3. **TLS поддержка** - использует сертификат `https.crt` для защищенного соединения
4. **Middleware для логирования** - логирует все запросы и ошибки 404

Этот сервер демонстрирует, как можно использовать protobuf для валидации без использования gRPC.
