package main

import (
	"fmt"
	"log"
	"net"

	amfpb "github.com/5g-core/proto/amf"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("AMF 启动失败: %v", err)
	}

	grpcServer := grpc.NewServer()

	// 注册 AMF 处理器
	amfpb.RegisterAMFServiceServer(grpcServer, NewAMFHandler())

	fmt.Println("AMF 服务启动，监听 :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("AMF 服务异常: %v", err)
	}
}
