# gRPC Test Project - Part 5: Complete TLS Architecture Summary

Этот документ предоставляет полный обзор архитектуры проекта с поддержкой TLS для всех сервисов.

## Обзор архитектуры

Проект демонстрирует различные способы интеграции gRPC с HTTP, все с полной поддержкой TLS:

```
┌─────────────────────────────────────────────────────────────────┐
│                         Клиенты                                  │
├─────────────────────────────────────────────────────────────────┤
│  gRPC Client (TLS)  │  HTTPS Clients (curl, browsers, etc.)    │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Точки входа                                │
├─────────────────────────────────────────────────────────────────┤
│  Envoy (HTTPS:18090)  │  OpenAPI (HTTPS:18081)  │  Proxy (HTTPS:18080)  │  HTTP-Server (HTTPS:18082)  │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                   gRPC Server (TLS:50051)                       │
└─────────────────────────────────────────────────────────────────┘
```

## TLS сертификаты

### Структура сертификатов

```
certs/
├── ca.crt              # Корневой сертификат (Certificate Authority)
├── ca.key              # Приватный ключ CA
├── server.crt          # Сертификат gRPC сервера
├── server.key          # Приватный ключ gRPC сервера
├── https.crt           # Сертификат для HTTPS серверов
└── https.key           # Приватный ключ для HTTPS серверов
```

### Детали сертификатов

**CA Certificate (ca.crt)**
- Валидность: 10 лет
- Используется для подписи всех других сертификатов
- Должен быть доверенным для всех клиентов

**Server Certificate (server.crt)**
- Subject: CN=server
- SAN: DNS:server, DNS:localhost, IP:127.0.0.1
- Валидность: 10 лет
- Используется: gRPC сервером

**HTTPS Certificate (https.crt)**
- Subject: CN=localhost
- SAN: DNS:localhost, DNS:proxy, DNS:openapi, DNS:http-server, DNS:envoy, IP:127.0.0.1, IP:0.0.0.0
- Валидность: 10 лет
- Используется: всеми HTTPS серверами (proxy, openapi, http-server, envoy)

## Конфигурация TLS по сервисам

### 1. gRPC Server (server:50051)

**Конфигурация:**
- Сертификат: `certs/server.crt`
- Приватный ключ: `certs/server.key`
- Минимальная версия TLS: 1.2
- Аутентификация клиента: Не требуется (может быть включена для mTLS)

**Код:**
```go
config := &tls.Config{
    Certificates: []tls.Certificate{serverCert},
    ClientAuth:   tls.NoClientCert,
    MinVersion:   tls.VersionTLS12,
}
```

### 2. gRPC Clients (proxy, openapi, envoy, client)

**Конфигурация:**
- CA сертификат: `certs/ca.crt`
- Проверка имени сервера: Включена
- Минимальная версия TLS: 1.2

**Код:**
```go
config := &tls.Config{
    RootCAs:            certPool,
    InsecureSkipVerify: false,
    MinVersion:         tls.VersionTLS12,
}
```

### 3. HTTPS Servers (proxy, openapi, http-server)

**Конфигурация:**
- Сертификат: `certs/https.crt`
- Приватный ключ: `certs/https.key`
- Минимальная версия TLS: 1.2

**Код:**
```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    MinVersion:   tls.VersionTLS12,
}

server := &http.Server{
    Addr:      ":8080",
    Handler:   nil,
    TLSConfig: tlsConfig,
}
```

### 4. Envoy Proxy

**Downstream TLS (HTTPS listener):**
```yaml
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
```

**Upstream TLS (gRPC backend):**
```yaml
transport_socket:
  name: envoy.transport_sockets.tls
  typed_config:
    "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
    common_tls_context:
      validation_context:
        trusted_ca:
          filename: "/etc/envoy/ca.crt"
```

## Потоки данных с TLS

### Поток 1: gRPC Client → gRPC Server

```
gRPC Client (TLS)
    │
    ├─ Загружает CA сертификат (ca.crt)
    ├─ Создает TLS credentials
    └─ Подключается к server:50051 с TLS
         │
         ▼
gRPC Server (TLS)
    ├─ Использует server.crt и server.key
    ├─ Валидирует TLS соединение
    └─ Обрабатывает gRPC запросы
```

### Поток 2: HTTPS Client → Proxy → gRPC Server

```
HTTPS Client (curl, browser)
    │
    ├─ Подключается к proxy:18080 с HTTPS
    └─ Отправляет JSON запрос
         │
         ▼
Proxy (HTTPS)
    ├─ Принимает HTTPS запрос (https.crt)
    ├─ Подключается к server:50051 с TLS (ca.crt)
    └─ Проксирует запрос в gRPC
         │
         ▼
gRPC Server (TLS)
    ├─ Обрабатывает gRPC запрос
    └─ Возвращает ответ
```

### Поток 3: HTTPS Client → OpenAPI Gateway → gRPC Server

```
HTTPS Client (curl, browser)
    │
    ├─ Подключается к openapi:18081 с HTTPS
    └─ Отправляет JSON запрос по REST API
         │
         ▼
OpenAPI Gateway (HTTPS)
    ├─ Принимает HTTPS запрос (https.crt)
    ├─ Транскодирует JSON в gRPC
    ├─ Подключается к server:50051 с TLS (ca.crt)
    └─ Отправляет gRPC запрос
         │
         ▼
gRPC Server (TLS)
    ├─ Обрабатывает gRPC запрос
    └─ Возвращает ответ
```

### Поток 4: HTTPS Client → Envoy → gRPC Server

```
HTTPS Client (curl, browser)
    │
    ├─ Подключается к envoy:18090 с HTTPS
    └─ Отправляет JSON запрос
         │
         ▼
Envoy (HTTPS)
    ├─ Принимает HTTPS запрос (https.crt)
    ├─ Транскодирует JSON в gRPC
    ├─ Подключается к server:50051 с TLS (ca.crt)
    └─ Отправляет gRPC запрос
         │
         ▼
gRPC Server (TLS)
    ├─ Обрабатывает gRPC запрос
    └─ Возвращает ответ
```

### Поток 5: HTTPS Client → HTTP Server

```
HTTPS Client (curl, browser)
    │
    ├─ Подключается к http-server:18082 с HTTPS
    └─ Отправляет JSON запрос
         │
         ▼
HTTP Server (HTTPS)
    ├─ Принимает HTTPS запрос (https.crt)
    ├─ Валидирует JSON через protobuf
    ├─ Хранит данные в памяти
    └─ Возвращает ответ
```

## Безопасность

### Рекомендации по безопасности

1. **Используйте сертификаты от доверенного CA** в продакшене
2. **Включите mTLS** для критичных сервисов
3. **Используйте TLS 1.3** где возможно
4. **Регулярно обновляйте сертификаты**
5. **Мониторьте истечение срока действия сертификатов**
6. **Используйте HSTS** для HTTPS серверов
7. **Ограничьте поддерживаемые шифры**

### Валидация сертификатов

Все клиенты валидируют сертификаты сервера:
- Проверка цепочки сертификатов
- Проверка имени сервера (SNI)
- Проверка срока действия
- Проверка отзыва (CRL/OCSP) - может быть добавлена

## Устранение неполадок

### Ошибки сертификатов

**Ошибка: `x509: certificate signed by unknown authority`**
- Убедитесь, что CA сертификат (ca.crt) установлен на клиенте
- Проверьте, что сертификат сервера подписан правильным CA

**Ошибка: `x509: certificate has expired or is not yet valid`**
- Проверьте срок действия сертификата
- Сгенерируйте новые сертификаты

**Ошибка: `x509: certificate specifies an incompatible key usage`**
- Убедитесь, что сертификат имеет правильные расширения (serverAuth/clientAuth)

### Ошибки подключения

**Ошибка: `connection refused`**
- Убедитесь, что сервис запущен
- Проверьте порты в docker-compose.yml
- Проверьте логи сервиса

**Ошибка: `no such host`**
- Убедитесь, что сервисы в одной сети Docker
- Проверьте имена сервисов в docker-compose.yml

## Резюме

Этот проект демонстрирует:

1. **Полный TLS стек** - все сервисы используют TLS
2. **Различные подходы к интеграции** - gRPC, HTTPS, Envoy, OpenAPI
3. **Валидация protobuf** - строгая проверка данных
4. **Безопасность** - TLS 1.2+, валидация сертификатов
5. **Гибкость** - можно использовать любой подход в зависимости от требований

Все сервисы готовы к использованию в продакшене с соответствующими сертификатами от доверенного CA.
