package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "net/http"
    "fmt"

//    "time"

    pb "grpc-app/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

var grpcClient pb.ChatServiceClient

func main() {
    // Подключаемся к gRPC серверу (в докере это будет хост 'server')

    grpcServerURL := os.Getenv("GRPC_SERVER_URL")

    // Set a default if empty
    if grpcServerURL == "" {
        grpcServerURL = "server:50051"
    }
    fmt.Println("gRPC Server URL:", grpcServerURL)


    conn, err := grpc.Dial(grpcServerURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("did not connect: %v", err)
    }
    defer conn.Close()
    grpcClient = pb.NewChatServiceClient(conn)

    http.HandleFunc("/api/send", handleSendMessage)
    http.HandleFunc("/api/get", handleGetMessage)
    http.HandleFunc("/api/version", handleApiVersion)

    log.Println("HTTP Proxy is running on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
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
