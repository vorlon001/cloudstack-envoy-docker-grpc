# gRPC Test Project - Part 3: Envoy Proxy with TLS

Envoy умеет делать **gRPC-JSON Transcoding** с поддержкой TLS.

Это значит, что Envoy сможет принимать **HTTPS запросы** от клиента, читать ваши `google.api.http` аннотации из скомпилированного proto-файла, на лету конвертировать JSON в gRPC и отправлять его на ваш `server` (порт 50051) с TLS. А ответ от `server` конвертировать обратно в JSON.

Для этого Envoy нужен скомпилированный бинарник вашего proto-файла (FileDescriptorSet).

### Шаг 1. Генерация дескриптора для Envoy

Создайте папку `envoy` в корне проекта. Затем выполните команду, которая скомпилирует ваш proto (вместе с зависимостями) в один бинарный файл `proto.pb`:

```bash
mkdir -p envoy
protoc -I proto -I third_party \
  --include_imports \
  --descriptor_set_out=envoy/proto.pb \
  proto/chat.proto
```
*В папке `envoy/` должен появиться файл `proto.pb`.*

### Шаг 2. Конфигурация Envoy с TLS (`envoy/envoy.yaml`)

Создайте файл `envoy/envoy.yaml`. Это конфиг, который говорит Envoy: "Слушай 8090 порт с TLS, ожидай JSON, переводить по правилам из `proto.pb` в gRPC и слать на `server:50051` с TLS".

```yaml
static_resources:
  listeners:
  - name: grpc_listener
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 8090
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          codec_type: AUTO
          stat_prefix: ingress_http
          # ВКЛЮЧАЕМ ЛОГИ ЧТОБЫ ВИДЕТЬ ЗАПРОСЫ
          access_log:
          - name: envoy.access_loggers.stdout
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
          route_config:
            name: local_route
            virtual_hosts:
            - name: backend
              domains:
              - "*"
              routes:
              - match:
                  prefix: "/"
                route:
                  cluster: grpc_backend
          http_filters:
          - name: envoy.filters.http.grpc_json_transcoder
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.grpc_json_transcoder.v3.GrpcJsonTranscoder
              proto_descriptor: "/etc/envoy/proto.pb"
              services:
              - chat.ChatService
              print_options:
                add_whitespace: true
                always_print_primitive_fields: true
                preserve_proto_field_names: true
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
      # TLS configuration for HTTPS listener
      transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
          common_tls_context:
            tls_certificates:
            - certificate_chain:
                filename: "/etc/envoy/https.crt"
              private_key:
                filename: "/etc/envoy/https.key"

  clusters:
  - name: grpc_backend
    connect_timeout: 5s
    type: STRICT_DNS
    lb_policy: ROUND_ROBIN
    # ВАЖНО: Для gRPC (HTTP/2) нужны именно эти опции
    typed_extension_protocol_options:
      envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
        "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
        explicit_http_config:
          http2_protocol_options: {}
    # TLS configuration for upstream gRPC server
    transport_socket:
      name: envoy.transport_sockets.tls
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
        common_tls_context:
          validation_context:
            trusted_ca:
              filename: "/etc/envoy/ca.crt"
    load_assignment:
      cluster_name: grpc_backend
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: server # Имя сервиса в docker-compose
                port_value: 50051
```

### Шаг 3. Обновляем `docker-compose.yml`

Добавьте сервис `envoy` в ваш `docker-compose.yml`:

```yaml
networks:
  app-network:
    driver: bridge

services:
  # ... (ваши сервисы server, proxy, openapi, client остаются без изменений) ...

  # ENVOY PROXY с TLS
  envoy:
    image: envoyproxy/envoy:v1.37-latest
    ports:
      - "18090:8090"
    volumes:
      - ./envoy/envoy.yaml:/etc/envoy/envoy.yaml:ro
      - ./envoy/proto.pb:/etc/envoy/proto.pb:ro
      - ./certs/https.crt:/etc/envoy/https.crt:ro
      - ./certs/https.key:/etc/envoy/https.key:ro
      - ./certs/ca.crt:/etc/envoy/ca.crt:ro
    depends_on:
      - server
    networks:
      - app-network
```

### Шаг 4. Тестирование

Запустите сервисы:
```bash
docker-compose up --build
```

Тестирование Envoy с TLS:

```bash
# Отправить сообщение (Envoy транскодирует JSON в gRPC)
curl -k -X POST https://localhost:18090/chat.ChatService/SendMessage \
  -H "Content-Type: application/json" \
  -d '{"username": "dave", "message": "Hello from Envoy!"}'

# Получить сообщения
curl -k https://localhost:18090/chat.ChatService/GetMessage \
  -H "Content-Type: application/json" \
  -d '{"username": "dave"}'

# Получить версию API
curl -k https://localhost:18090/chat.ChatService/ApiVersion \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Резюме

В этом руководстве мы добавили:
1. **Envoy Proxy с TLS** - принимает HTTPS запросы и транскодирует их в gRPC с TLS
2. **Downstream TLS** - использует сертификат `https.crt` для HTTPS соединения с клиентами
3. **Upstream TLS** - использует CA сертификат для валидации gRPC сервера
4. **gRPC-JSON Transcoding** - автоматически конвертирует JSON в gRPC и обратно

Envoy теперь работает как полноценный HTTPS→gRPC(TLS) прокси с валидацией protobuf схем.
