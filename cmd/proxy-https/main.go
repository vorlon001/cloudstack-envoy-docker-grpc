package main

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "io/ioutil"
    "log"
    "os"
    "fmt"

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
    // Подключаемся к gRPC серверу с TLS (в докере это будет хост 'server')
    creds, err := loadTLSCredentials()
    if err != nil {
        log.Fatalf("failed to load TLS credentials: %v", err)
    }

    grpcServerURL := os.Getenv("GRPC_SERVER_URL")

    // Set a default if empty
    if grpcServerURL == "" {
        grpcServerURL = "server:50051"
    }
    fmt.Println("gRPC Server URL:", grpcServerURL)

    conn, err := grpc.Dial(grpcServerURL, grpc.WithTransportCredentials(creds))
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
