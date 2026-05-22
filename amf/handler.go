package main

import (
	"context"
	"fmt"
	"log"

	amfpb "github.com/5g-core/proto/amf"
)

// AMFHandler 实现 AMFService 接口
type AMFHandler struct {
	amfpb.UnimplementedAMFServiceServer
	// 存储已注册的 UE，key 是 ue_id，value 是 amf_ue_id
	registeredUEs map[string]string
}

func NewAMFHandler() *AMFHandler {
	return &AMFHandler{
		registeredUEs: make(map[string]string),
	}
}

// Register 处理 UE 注册请求
func (h *AMFHandler) Register(ctx context.Context, req *amfpb.RegisterRequest) (*amfpb.RegisterResponse, error) {
	log.Printf("收到注册请求: UE_ID=%s, 信号强度=%.2f dBm, SINR=%.2f dB",
		req.UeId, req.SignalPower, req.Sinr)

	// 检查信道质量，信号太差拒绝注册
	if req.SignalPower < -110 {
		return &amfpb.RegisterResponse{
			Success: false,
			Message: fmt.Sprintf("信号强度不足 (%.2f dBm)，拒绝注册", req.SignalPower),
		}, nil
	}

	// 分配一个 AMF 内部 ID
	amfUeID := fmt.Sprintf("AMF-%s-001", req.UeId)
	h.registeredUEs[req.UeId] = amfUeID

	log.Printf("注册成功: UE_ID=%s -> AMF_UE_ID=%s", req.UeId, amfUeID)

	return &amfpb.RegisterResponse{
		Success: true,
		AmfUeId: amfUeID,
		Message: "注册成功",
	}, nil
}

// Deregister 处理 UE 去注册请求
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
