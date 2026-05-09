# gRPC Test Project with TLS

This project demonstrates various ways to integrate gRPC with HTTP, now with full TLS support for all services.

## Architecture

The project consists of the following services:

| Service | Port (host) | Protocol | Description |
|---------|-------------|----------|-------------|
| server | 50051 | gRPC+TLS | Core gRPC server with TLS |
| proxy | 18080 | HTTPS | Custom HTTP→gRPC proxy with TLS |
| openapi | 18081 | HTTPS | OpenAPI Gateway (grpc-gateway v2) with TLS |
| http-server | 18082 | HTTPS | Pure HTTP server with protobuf validation and TLS |
| client | - | gRPC+TLS | gRPC client with TLS (one-time execution) |
| envoy | 18090 | HTTPS | Envoy Proxy with TLS |

## TLS Setup

### Certificate Generation

Before running the project, generate TLS certificates:

```bash
cd grpc.test.5/certs
bash generate-certs.sh
```

This script creates the following certificates:

- `ca.crt` - Certificate Authority (CA) certificate
- `server.crt` + `server.key` - Server certificate for gRPC server
- `client.crt` + `client.key` - Client certificate for mutual TLS (optional)
- `https.crt` + `https.key` - HTTPS certificate for HTTP servers

### Certificate Details

**CA Certificate:**
- Valid for 10 years
- Used to sign all other certificates

**Server Certificate:**
- Subject: CN=server
- SAN: DNS:server, DNS:localhost, IP:127.0.0.1
- Valid for 10 years

**HTTPS Certificate:**
- Subject: CN=localhost
- SAN: DNS:localhost, DNS:proxy, DNS:openapi, DNS:http-server, DNS:envoy, IP:127.0.0.1, IP:0.0.0.0
- Valid for 10 years

## Running the Project

### Using Docker Compose

```bash
cd grpc.test.5
docker-compose up --build
```

### Using Make

```bash
# Generate certificates first
cd grpc.test.5/certs
bash generate-certs.sh

# Build all binaries
cd ..
make build

# Run with docker-compose
make docker-up
```

## Testing the Services

### 1. gRPC Server (with TLS)

```bash
# Using grpcurl
grpcurl -insecure -cert certs/client.crt -key certs/client.key \
  server:50051 chat.ChatService/ApiVersion

# Using the gRPC client
docker-compose run client
```

### 2. HTTPS Proxy

```bash
# Send a message
curl -k -X POST https://localhost:18080/api/send \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "message": "Hello from HTTPS!"}'

# Get messages
curl -k https://localhost:18080/api/get?username=alice

# Get API version
curl -k https://localhost:18080/api/version
```

### 3. OpenAPI Gateway (with TLS)

```bash
# Get OpenAPI spec
curl -k https://localhost:18081/openapi.yaml

# Send a message
curl -k -X POST https://localhost:18081/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"username": "bob", "message": "Hello from OpenAPI!"}'

# Get messages
curl -k https://localhost:18081/v1/messages/bob

# Get API version
curl -k https://localhost:18081/v1/version
```

### 4. Pure HTTP Server (with TLS)

```bash
# Send a message
curl -k -X POST https://localhost:18082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"username": "charlie", "message": "Hello from HTTP Server!"}'

# Get messages
curl -k https://localhost:18082/v1/messages/charlie

# Get API version
curl -k https://localhost:18082/v1/version
```

### 5. Envoy Proxy (with TLS)

```bash
# Send a message (Envoy transcodes JSON to gRPC)
curl -k -X POST https://localhost:18090/chat.ChatService/SendMessage \
  -H "Content-Type: application/json" \
  -d '{"username": "dave", "message": "Hello from Envoy!"}'

# Get messages
curl -k https://localhost:18090/chat.ChatService/GetMessage \
  -H "Content-Type: application/json" \
  -d '{"username": "dave"}'

# Get API version
curl -k https://localhost:18090/chat.ChatService/ApiVersion \
  -H "Content-Type: application/json" \
  -d '{}'
```

## TLS Configuration Details

### gRPC Server

The gRPC server uses TLS with:
- Server certificate: `certs/server.crt`
- Server private key: `certs/server.key`
- Minimum TLS version: 1.2
- Client authentication: Not required (can be enabled for mTLS)

### gRPC Clients (proxy, openapi, envoy, client)

All gRPC clients use TLS with:
- CA certificate: `certs/ca.crt`
- Server name verification: Enabled
- Minimum TLS version: 1.2

### HTTPS Servers (proxy, openapi, http-server)

All HTTPS servers use TLS with:
- Server certificate: `certs/https.crt`
- Server private key: `certs/https.key`
- Minimum TLS version: 1.2

### Envoy Proxy

Envoy is configured with:
- Downstream TLS (HTTPS listener): Uses `https.crt` and `https.key`
- Upstream TLS (gRPC backend): Uses `ca.crt` for server verification
- HTTP/2 support for gRPC

## Project Structure

```
grpc.test.5/
├── certs/                    # TLS certificates
│   ├── generate-certs.sh     # Certificate generation script
│   ├── ca.crt                # CA certificate
│   ├── server.crt            # Server certificate
│   ├── server.key            # Server private key
│   ├── https.crt             # HTTPS certificate
│   └── https.key             # HTTPS private key
├── cmd/                      # Application binaries
│   ├── server/               # gRPC server with TLS
│   ├── proxy/                # HTTPS→gRPC proxy with TLS
│   ├── client/               # gRPC client with TLS
│   ├── openapi-server/       # OpenAPI gateway with TLS
│   └── http-server/          # Pure HTTP server with TLS
├── proto/                    # Protocol Buffer definitions
├── envoy/                    # Envoy configuration
│   ├── envoy.yaml            # Envoy config with TLS
│   └── proto.pb              # Compiled proto descriptor
├── third_party/              # Third-party proto files
├── docker-compose.yml        # Docker Compose configuration
├── Dockerfile                # Multi-stage Dockerfile
├── Makefile                  # Build automation
└── go.mod                    # Go module definition
```

## Security Notes

1. **Self-signed certificates**: This project uses self-signed certificates for demonstration purposes. In production, use certificates from a trusted CA.

2. **Certificate validation**: All clients validate the server certificate using the CA certificate.

3. **TLS version**: Minimum TLS version is set to 1.2 for all services.

4. **InsecureSkipVerify**: The `-k` flag in curl commands skips certificate verification for testing. In production, always verify certificates.

## Troubleshooting

### Certificate errors

If you see certificate-related errors:
1. Ensure certificates are generated: `cd certs && bash generate-certs.sh`
2. Check certificate paths in docker-compose.yml
3. Verify certificate permissions

### Connection refused

If services can't connect:
1. Ensure all services are running: `docker-compose ps`
2. Check service logs: `docker-compose logs <service>`
3. Verify network configuration in docker-compose.yml

### Envoy connection issues

If Envoy can't connect to the gRPC server:
1. Check that the gRPC server is running with TLS
2. Verify CA certificate is mounted correctly
3. Check Envoy logs: `docker-compose logs envoy`

## Development

### Building individual services

```bash
make server      # Build gRPC server
make proxy       # Build HTTPS proxy
make client      # Build gRPC client
make openapi-server  # Build OpenAPI gateway
make http-server     # Build HTTP server
```

### Cleaning up

```bash
make clean       # Remove binary files
make docker-down # Stop Docker services
```

## References

- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [grpc-gateway Documentation](https://grpc-ecosystem.github.io/grpc-gateway/)
- [Envoy Documentation](https://www.envoyproxy.io/docs/)
- [Protocol Buffers](https://developers.google.com/protocol-buffers)
