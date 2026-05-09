package main

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "io/ioutil"
    "log"
    "os"
    "fmt"
    "time"

    pb "grpc-app/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
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
    // Ждем 2 секунды, чтобы сервер в докере точно успел подняться
    time.Sleep(2 * time.Second)

    // Load TLS credentials
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
    log.Printf("Messages for alice: `%v`", getRes.GetMessages())
}
