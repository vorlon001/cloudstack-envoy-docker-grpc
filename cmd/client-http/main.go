package main

import (
    "context"
    "log"
    "time"
    "os"
    "fmt"

    pb "grpc-app/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // Ждем 2 секунды, чтобы сервер в докере точно успел подняться
    time.Sleep(2 * time.Second)

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

    client := pb.NewChatServiceClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
    defer cancel()

    // 1. Проверяем версию
    ver, err := client.ApiVersion(ctx, &pb.ApiVersionRequest{})
    if err != nil {
        log.Fatalf("ApiVersion failed: %v", err)
    }
    log.Printf("API Version: %s", ver.GetVersion())

    // 2. Отправляем сообщение
    sendRes, err := client.SendMessage(ctx, &pb.SendMessageRequest{Username: "alice", Message: "Hello from gRPC client! 1"})
    if err != nil {
        log.Fatalf("SendMessage failed: %v", err)
    }
    log.Printf("SendMessage Success: %v", sendRes.GetSuccess())

    // 2. Отправляем сообщение
    sendRes, err = client.SendMessage(ctx, &pb.SendMessageRequest{Username: "alice", Message: "Hello from gRPC client! 2"})
    if err != nil {
        log.Fatalf("SendMessage failed: %v", err)
    }
    log.Printf("SendMessage Success: %v", sendRes.GetSuccess())

    // 2. Отправляем сообщение
    sendRes, err = client.SendMessage(ctx, &pb.SendMessageRequest{Username: "alice", Message: "Hello from gRPC client! 3"})
    if err != nil {
        log.Fatalf("SendMessage failed: %v", err)
    }
    log.Printf("SendMessage Success: %v", sendRes.GetSuccess())



    // 3. Получаем сообщения
    getRes, err := client.GetMessage(ctx, &pb.GetMessageRequest{Username: "alice"})
    if err != nil {
        log.Fatalf("GetMessage failed: %v", err)
    }
    log.Printf("Messages for alice: %v", getRes.GetMessages())
}
