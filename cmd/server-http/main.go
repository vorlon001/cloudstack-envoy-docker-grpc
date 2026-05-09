package main

import (
    "context"
    "log"
    "net"
    "sync"

    pb "grpc-app/proto"
    "google.golang.org/grpc"
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

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    s := grpc.NewServer()
    pb.RegisterChatServiceServer(s, newServer())
    log.Println("gRPC Server is running on :50051")
    if err := s.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
