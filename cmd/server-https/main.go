package main

import (
    "context"
    "crypto/tls"
    "log"
    "net"
    "sync"

    pb "grpc-app/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

type server struct {
    pb.UnimplementedChatServiceServer
    mu       sync.RWMutex
    messages map[string][]string
}

func newServer() *server {
    return &server{messages: make(map[string][]string)}
}

func (s *server) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.messages[req.Username] = append(s.messages[req.Username], req.Message)
    return &pb.SendMessageResponse{Success: true}, nil
}

func (s *server) GetMessage(ctx context.Context, req *pb.GetMessageRequest) (*pb.GetMessageResponse, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    msgs := s.messages[req.Username]
    if msgs == nil {
        msgs = []string{}
    }
    return &pb.GetMessageResponse{Messages: msgs}, nil
}

func (s *server) ApiVersion(ctx context.Context, req *pb.ApiVersionRequest) (*pb.ApiVersionResponse, error) {
    return &pb.ApiVersionResponse{Version: "1.0.0"}, nil
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
    // Load server's certificate and private key
    serverCert, err := tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
    if err != nil {
        return nil, err
    }

    // Create the credentials and return it
    config := &tls.Config{
        Certificates: []tls.Certificate{serverCert},
        ClientAuth:   tls.NoClientCert, // No client cert required for basic TLS
        MinVersion:   tls.VersionTLS12,
    }

    return credentials.NewTLS(config), nil
}

func main() {
    // Load TLS credentials
    creds, err := loadTLSCredentials()
    if err != nil {
        log.Fatalf("failed to load TLS credentials: %v", err)
    }

    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    
    s := grpc.NewServer(grpc.Creds(creds))
    pb.RegisterChatServiceServer(s, newServer())
    log.Println("gRPC Server with TLS is running on :50051")
    if err := s.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
