package main

import (
	"context"
	"fmt"
	"log"
	"time"

	amfpb "github.com/5g-core/proto/amf"
	smfpb "github.com/5g-core/proto/smf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AMFHandler struct {
	amfpb.UnimplementedAMFServiceServer
	registeredUEs map[string]string // ue_id -> amf_ue_id
	smfClient     smfpb.SMFServiceClient
}

func NewAMFHandler() *AMFHandler {
	// AMF 启动时连接 SMF
	conn, err := grpc.NewClient("localhost:50052",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("AMF 连接 SMF 失败: %v", err)
	}

	return &AMFHandler{
		registeredUEs: make(map[string]string),
		smfClient:     smfpb.NewSMFServiceClient(conn),
	}
}

func (h *AMFHandler) Register(ctx context.Context, req *amfpb.RegisterRequest) (*amfpb.RegisterResponse, error) {
	log.Printf("收到注册请求: UE_ID=%s, 信号强度=%.2f dBm, SINR=%.2f dB",
		req.UeId, req.SignalPower, req.Sinr)

	// 检查信道质量
	if req.SignalPower < -110 {
		return &amfpb.RegisterResponse{
			Success: false,
			Message: fmt.Sprintf("信号强度不足 (%.2f dBm)，拒绝注册", req.SignalPower),
		}, nil
	}

	// 分配 AMF 内部 ID
	amfUeID := fmt.Sprintf("AMF-%s-001", req.UeId)
	h.registeredUEs[req.UeId] = amfUeID
	log.Printf("注册成功: UE_ID=%s -> AMF_UE_ID=%s", req.UeId, amfUeID)

	// 注册成功后，自动请求 SMF 建立会话
	smfCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	smfResp, err := h.smfClient.CreateSession(smfCtx, &smfpb.CreateSessionRequest{
		UeId:    req.UeId,
		AmfUeId: amfUeID,
		Dnn:     "internet",
	})
	if err != nil {
		log.Printf("AMF 请求 SMF 建立会话失败: %v", err)
	} else if smfResp.Success {
		log.Printf("SMF 会话建立成功: SESSION_ID=%s, UE_IP=%s",
			smfResp.SessionId, smfResp.UeIp)
	}

	return &amfpb.RegisterResponse{
		Success: true,
		AmfUeId: amfUeID,
		Message: fmt.Sprintf("注册成功，已分配IP: %s", smfResp.UeIp),
	}, nil
}

func (h *AMFHandler) Deregister(ctx context.Context, req *amfpb.DeregisterRequest) (*amfpb.DeregisterResponse, error) {
	log.Printf("收到去注册请求: UE_ID=%s", req.UeId)

	if _, exists := h.registeredUEs[req.UeId]; !exists {
		return &amfpb.DeregisterResponse{
			Success: false,
			Message: "UE 不存在",
		}, nil
	}

	delete(h.registeredUEs, req.UeId)
	log.Printf("去注册成功: UE_ID=%s", req.UeId)

	return &amfpb.DeregisterResponse{
		Success: true,
		Message: "去注册成功",
	}, nil
}
