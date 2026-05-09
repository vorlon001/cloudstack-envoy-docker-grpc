package main

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "io/ioutil"
    "log"
    "os"
    "fmt"
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

    grpcServerURL := os.Getenv("GRPC_SERVER_URL")

    // Set a default if empty
    if grpcServerURL == "" {
        grpcServerURL = "server:50051"
    }
    fmt.Println("gRPC Server URL:", grpcServerURL)

    opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
    err = pb.RegisterChatServiceHandlerFromEndpoint(ctx, grpcGatewayMux, grpcServerURL, opts)
    if err != nil {
        log.Fatalf("Failed to register gateway: %v", err)
    }

    // 3. Привязываем gRPC шлюз к корню
    rootMux.Handle("/", grpcGatewayMux)

    // 4. Привязываем отдачу файла openapi.yaml (теперь это работает без ошибок сигнатуры)
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
