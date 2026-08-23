package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	smfpb "github.com/5g-core/proto/smf"
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
func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err == nil && value > 0 {
		return value
	}
	return fallback
}

func main() {
	listenAddress := env("LISTEN_ADDRESS", ":50052")
	handler, err := NewSMFHandler(env("UPF_ADDRESS", "localhost:50053"), 3*time.Second, envInt("IP_POOL_SIZE", 254))
	if err != nil {
		log.Fatalf("初始化 SMF: %v", err)
	}
	defer handler.Close()
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("SMF 监听失败: %v", err)
	}
	server := grpc.NewServer()
	smfpb.RegisterSMFServiceServer(server, handler)
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, hs)
	go func() {
		log.Printf("SMF 服务启动 listen=%s upf=%s", listenAddress, env("UPF_ADDRESS", "localhost:50053"))
		if err := server.Serve(lis); err != nil {
			log.Fatalf("SMF 服务异常: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	server.GracefulStop()
}
