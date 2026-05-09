package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "fmt"


    "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    pb "grpc-app/proto"
)

func main() {
    ctx := context.Background()

    // 1. Создаем СТАНДАРТНЫЙ http мультиплексор для статики (openapi.yaml)
    rootMux := http.NewServeMux()

    // 2. Создаем мультиплексор grpc-gateway v2 для API
    grpcGatewayMux := runtime.NewServeMux()


    grpcServerURL := os.Getenv("GRPC_SERVER_URL")

    // Set a default if empty
    if grpcServerURL == "" {
        grpcServerURL = "server:50051"
    }
    fmt.Println("gRPC Server URL:", grpcServerURL)


    // Подключаемся к gRPC серверу
    opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
    err := pb.RegisterChatServiceHandlerFromEndpoint(ctx, grpcGatewayMux, grpcServerURL, opts)
    if err != nil {
        log.Fatalf("Failed to register gateway: %v", err)
    }

    // 3. Привязываем gRPC шлюз к корню
    rootMux.Handle("/", grpcGatewayMux)

    // 4. Привязываем отдачу файла openapi.yaml (теперь это работает без ошибок сигнатуры)
    rootMux.Handle("/openapi.yaml", http.FileServer(http.Dir("./")))

    log.Println("OpenAPI Gateway Server (v2) is running on :8081")
    log.Println("Spec URL: http://localhost:8081/openapi.yaml")
    log.Println("Swagger UI: https://petstore.swagger.io/?url=http://localhost:8081/openapi.yaml")

    if err := http.ListenAndServe("0.0.0.0:8081", rootMux); err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }
}
