```

Как запустить
cd grpc.test.5/certs
bash generate-certs.sh


make fetch-apis
make build
docker-compose build
docker-compose up -d

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0


cd ..
docker-compose up --build

Тестирование

# TEST openapi-http
curl "http://localhost:18101/v1/messages/bob"
curl -X POST http://localhost:18101/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!222"}'
curl "http://localhost:18101/v1/messages/bob"


# TEST http-server
curl "http://localhost:18083/v1/messages/bob"
curl -X POST http://localhost:18083/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!333"}'
curl "http://localhost:18083/v1/messages/bob"

# TEST proxy-http
curl "http://localhost:18100/api/version"
curl -X POST http://localhost:18100/api/send      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!333"}'
curl "http://localhost:18100/api/get?username=bob"


# TEST server-http
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

grpcurl -plaintext -import-path third_party -import-path proto -proto proto/chat.proto -d '{"username": "bob"}' localhost:50052 chat.ChatService/GetMessage
grpcurl -plaintext -import-path third_party -import-path proto -proto proto/chat.proto -d '{"username": "bob", "message": "via envoy"}' localhost:50052 chat.ChatService/SendMessage
grpcurl -plaintext -import-path third_party -import-path proto -proto proto/chat.proto -d '{"username": "bob"}' localhost:50052 chat.ChatService/GetMessage

# TEST envoy-http
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

grpcurl -plaintext -import-path third_party -import-path proto -proto proto/chat.proto -d '{"username": "bob"}' localhost:18109 chat.ChatService/GetMessage
grpcurl -plaintext -import-path third_party -import-path proto -proto proto/chat.proto -d '{"username": "bob", "message": "via envoy"}' localhost:18109 chat.ChatService/SendMessage
grpcurl -plaintext -import-path third_party -import-path proto -proto proto/chat.proto -d '{"username": "bob"}' localhost:18109 chat.ChatService/GetMessage


# HTTPS Proxy
curl -k -X POST https://localhost:18080/api/send -H "Content-Type: application/json" -d '{"username": "alice", "message": "Hello!"}'
 curl -k -X GET https://localhost:18080/api/get?username=alice -H "Content-Type: application/json"

# OpenAPI Gateway
curl -k -X POST https://localhost:18081/v1/messages -H "Content-Type: application/json" -d '{"username": "bob", "message": "Hello!"}'
curl -k -X GET https://localhost:18081/v1/messages/bob -H "Content-Type: application/json"

# HTTPS Server
curl -k -X POST https://localhost:18082/v1/messages -H "Content-Type: application/json" -d '{"username": "charlie", "message": "Hello!"}'
curl -k -X GET https://localhost:18082/v1/messages/charlie -H "Content-Type: application/json"

# HTTP SERVER

curl  -X POST http://localhost:18083/v1/messages -H "Content-Type: application/json" -d '{"username": "charlie", "message": "Hello!"}'
curl -X GET http://localhost:18083/v1/messages/charlie -H "Content-Type: application/json"

# Envoy
curl -k -X POST https://localhost:18090/chat.ChatService/SendMessage -H "Content-Type: application/json" -d '{"username": "dave", "message": "Hello!"}'
```


# TEST ENVOY LBAAS
```
# TEST http-server
curl  -H "Host: api.example.com" -X POST http://localhost:18110/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!333"}'
curl  -H "Host: api.example.com" "http://localhost:18110/v1/messages/bob"

# HTTPS Proxy

curl -k -H "Host: api.example.com" -X POST https://localhost:18111/v1/messages -H "Content-Type: application/json" -d '{"username": "bob", "message": "Hello!88888"}'
curl -k -H "Host: api.example.com" -X GET https://localhost:18111/v1/messages/bob -H "Content-Type: application/json"

curl -k -H "Host: apis.example.com" -X POST https://localhost:18111/v1/messages -H "Content-Type: application/json" -d '{"username": "bob", "message": "Hello!4444"}'
curl -k -H "Host: apis.example.com" -X GET https://localhost:18111/v1/messages/bob -H "Content-Type: application/json"


# OpenAPI Gateway
curl -H "Host: apis2.example.com" -X POST http://localhost:18110/v1/messages -H "Content-Type: application/json" -d '{"username": "bob", "message": "Hello555!"}'
curl  -H "Host: apis2.example.com" -X GET http://localhost:18110/v1/messages/bob -H "Content-Type: application/json"

# TEST openapi-http
curl -H "Host: apis3.example.com" -X POST http://localhost:18110/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!666"}'
curl -H "Host: apis3.example.com" "http://localhost:18110/v1/messages/bob"

# OpenAPI Gateway
curl -k -H "Host: apis2.example.com" -X POST https://localhost:18111/v1/messages -H "Content-Type: application/json" -d '{"username": "bob", "message": "Hello555!"}'
curl -k -H "Host: apis2.example.com" -X GET https://localhost:18111/v1/messages/bob -H "Content-Type: application/json"


# TEST openapi-http
curl -k -H "Host: apis3.example.com" -X POST https://localhost:18111/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!666"}'
curl -k -H "Host: apis3.example.com" "https://localhost:18111/v1/messages/bob"


# test in grpc server
grpcurl -plaintext -import-path third_party -import-path proto -proto proto/chat.proto -d '{"username": "bob"}' localhost:18109 chat.ChatService/GetMessage

grpcurl -insecure \
  -cert certs/client.crt\
  -key certs/client.key\
  -proto chat.proto\
  -import-path ./proto -import-path ./third_party \
  -d '{"username": "bob"}' \
  localhost:18090 chat.ChatService/GetMessage

echo | openssl s_client -connect 127.0.0.1:18111 -servername api.example.com 2>/dev/null | openssl x509 -noout -text

```

# TEST ENVOY LBAAS TCP
```

curl -X POST http://localhost:18120/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!222"}'
curl -X POST http://localhost:18120/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!222"}'
curl -X POST http://localhost:18120/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!222"}'
curl -X POST http://localhost:18120/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!222"}'
curl -X POST http://localhost:18120/v1/messages      -H "Content-Type: application/json"      -d '{"username": "bob", "message": "Hello via HTTP!222"}'
curl "http://localhost:18120/v1/messages/bob"
```


**1. Установите grpcurl (если нет):**
```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

```

grpcurl -insecure \
  -cert certs/client.crt \
  -key certs/client.key \
  -proto chat.proto \
  -import-path ./proto -import-path ./third_party \
  -d '{"username": "bob", "message": "Hello via Envoy gRPC!"}' \
  localhost:50051 chat.ChatService/SendMessage

grpcurl -insecure \
  -cert certs/client.crt\
  -key certs/client.key\
  -proto chat.proto\
  -import-path ./proto -import-path ./third_party \
  -d '{"username": "bob"}' \
  localhost:50051 chat.ChatService/GetMessage


```



```

grpcurl -insecure \
  -cert certs/client.crt \
  -key certs/client.key \
  -proto chat.proto \
  -import-path ./proto -import-path ./third_party \
  -d '{"username": "bob", "message": "Hello via Envoy gRPC!"}' \
  localhost:18090 chat.ChatService/SendMessage

grpcurl -insecure \
  -cert certs/client.crt\
  -key certs/client.key\
  -proto chat.proto\
  -import-path ./proto -import-path ./third_party \
  -d '{"username": "bob"}' \
  localhost:18090 chat.ChatService/GetMessage


```



