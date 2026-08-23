package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	amfpb "github.com/5g-core/proto/amf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	listenAddress := env("LISTEN_ADDRESS", ":50051")
	handler, err := NewAMFHandler(env("SMF_ADDRESS", "localhost:50052"), 3*time.Second)
	if err != nil {
		log.Fatalf("初始化 AMF: %v", err)
	}
	defer handler.Close()
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("AMF 监听失败: %v", err)
	}
	server := grpc.NewServer()
	amfpb.RegisterAMFServiceServer(server, handler)
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, hs)
	go func() {
		log.Printf("AMF 服务启动 listen=%s smf=%s", listenAddress, env("SMF_ADDRESS", "localhost:50052"))
		if err := server.Serve(lis); err != nil {
			log.Fatalf("AMF 服务异常: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	server.GracefulStop()
}
