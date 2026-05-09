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
DNS.2 = server-https
DNS.3 = localhost
IP.1 = 127.0.0.1
EOF

openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr -config server.conf
openssl x509 -req -days 3650 -in server.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out server.crt -extensions v3_req -extfile server.conf

# 3. Generate client certificate (for proxy, openapi-server, envoy)
echo "==> Generating client certificate..."
cat > client.conf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = RU
ST = Test
L = Test
O = Test
OU = Client
CN = client-https

[v3_req]
keyUsage = digitalSignature
extendedKeyUsage = clientAuth
EOF

openssl genrsa -out client.key 4096
openssl req -new -key client.key -out client.csr -config client.conf
openssl x509 -req -days 3650 -in client.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out client.crt -extensions v3_req -extfile client.conf

# 4. Generate HTTPS certificate for HTTP servers (proxy, openapi-server, http-server)
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
DNS.3 = proxy-https
DNS.4 = openapi
DNS.5 = openapi-https
DNS.6 = http-server
DNS.7 = envoy
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

openssl genrsa -out https.key 4096
openssl req -new -key https.key -out https.csr -config https.conf
openssl x509 -req -days 3650 -in https.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out https.crt -extensions v3_req -extfile https.conf

# Clean up temporary files
rm -f server.csr client.csr https.csr server.conf client.conf https.conf ca.srl

echo "==> Certificates generated successfully!"
echo "==> Files created:"
ls -la




chmod 666 *crt
chmod 666 *key
