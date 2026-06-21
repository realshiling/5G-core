package main

import (
	"context"
	"fmt"
	"log"
	"time"

	smfpb "github.com/5g-core/proto/smf"
	upfpb "github.com/5g-core/proto/upf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SMFHandler struct {
	smfpb.UnimplementedSMFServiceServer
	sessions  map[string]string // session_id -> ue_ip
	ipPool    []string
	ipIndex   int
	upfClient upfpb.UPFServiceClient
}

func NewSMFHandler() *SMFHandler {
	// SMF 启动时连接 UPF
	conn, err := grpc.NewClient("localhost:50053",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("SMF 连接 UPF 失败: %v", err)
	}

	return &SMFHandler{
		sessions: make(map[string]string),
		ipPool: []string{
			"10.0.0.1", "10.0.0.2", "10.0.0.3",
			"10.0.0.4", "10.0.0.5", "10.0.0.6",
		},
		ipIndex:   0,
		upfClient: upfpb.NewUPFServiceClient(conn),
	}
}

func (h *SMFHandler) CreateSession(ctx context.Context, req *smfpb.CreateSessionRequest) (*smfpb.CreateSessionResponse, error) {
	log.Printf("收到建立会话请求: UE_ID=%s, DNN=%s", req.UeId, req.Dnn)

	if h.ipIndex >= len(h.ipPool) {
		return &smfpb.CreateSessionResponse{
			Success: false,
			Message: "IP地址池已耗尽",
		}, nil
	}

	// 分配IP和会话ID
	ueIP := h.ipPool[h.ipIndex]
	h.ipIndex++
	sessionID := fmt.Sprintf("SMF-SESSION-%s-%d", req.UeId, h.ipIndex)
	h.sessions[sessionID] = ueIP

	log.Printf("会话建立成功: UE_ID=%s, SESSION_ID=%s, UE_IP=%s",
		req.UeId, sessionID, ueIP)

	// 通知 UPF 下发转发规则
	upfCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	upfResp, err := h.upfClient.CreateRule(upfCtx, &upfpb.CreateRuleRequest{
		SessionId: sessionID,
		UeId:      req.UeId,
		UeIp:      ueIP,
		Dnn:       req.Dnn,
	})
	if err != nil {
		log.Printf("SMF 通知 UPF 失败: %v", err)
	} else if upfResp.Success {
		log.Printf("UPF 转发规则下发成功: RULE_ID=%s", upfResp.RuleId)
	}

	return &smfpb.CreateSessionResponse{
		Success:   true,
		SessionId: sessionID,
		UeIp:      ueIP,
		Message:   "会话建立成功",
	}, nil
}

func (h *SMFHandler) DeleteSession(ctx context.Context, req *smfpb.DeleteSessionRequest) (*smfpb.DeleteSessionResponse, error) {
	log.Printf("收到删除会话请求: SESSION_ID=%s", req.SessionId)

	if _, exists := h.sessions[req.SessionId]; !exists {
		return &smfpb.DeleteSessionResponse{
			Success: false,
			Message: "会话不存在",
		}, nil
	}

	delete(h.sessions, req.SessionId)
	log.Printf("会话删除成功: SESSION_ID=%s", req.SessionId)

	return &smfpb.DeleteSessionResponse{
		Success: true,
		Message: "会话删除成功",
	}, nil
}
