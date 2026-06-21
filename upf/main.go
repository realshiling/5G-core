package main

import (
	"fmt"
	"log"
	"net"

	upfpb "github.com/5g-core/proto/upf"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("UPF 启动失败: %v", err)
	}

	grpcServer := grpc.NewServer()
	upfpb.RegisterUPFServiceServer(grpcServer, NewUPFHandler())

	fmt.Println("UPF 服务启动，监听 :50053")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("UPF 服务异常: %v", err)
	}
}
