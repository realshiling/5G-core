package main

import (
	"fmt"
	"log"
	"net"

	smfpb "github.com/5g-core/proto/smf"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("SMF 启动失败: %v", err)
	}

	grpcServer := grpc.NewServer()
	smfpb.RegisterSMFServiceServer(grpcServer, NewSMFHandler())

	fmt.Println("SMF 服务启动，监听 :50052")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("SMF 服务异常: %v", err)
	}
}
