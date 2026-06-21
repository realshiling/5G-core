package main

import (
	"context"
	"log"
	"time"

	amfpb "github.com/5g-core/proto/amf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 连接 AMF
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("连接 AMF 失败: %v", err)
	}
	defer conn.Close()

	client := amfpb.NewAMFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 发起注册请求，携带信道质量数据
	resp, err := client.Register(ctx, &amfpb.RegisterRequest{
		UeId:        "UE-001",
		UeType:      "smartphone",
		SignalPower: -85.0, // 信号强度 dBm，高于-110所以能注册成功
		Sinr:        15.0,  // 信噪比 dB
	})
	if err != nil {
		log.Fatalf("注册请求失败: %v", err)
	}

	if resp.Success {
		log.Printf("✅ 注册成功！AMF_UE_ID: %s", resp.AmfUeId)
	} else {
		log.Printf("❌ 注册被拒绝: %s", resp.Message)
	}
}
