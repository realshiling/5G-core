package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	amfpb "github.com/5g-core/proto/amf"
	smfpb "github.com/5g-core/proto/smf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type UERecord struct {
	UEID, AMFUEID, SessionID, UEIP, State string
}

type AMFHandler struct {
	amfpb.UnimplementedAMFServiceServer
	mu            sync.RWMutex
	registeredUEs map[string]UERecord
	smfClient     smfpb.SMFServiceClient
	conn          *grpc.ClientConn
	callTimeout   time.Duration
}

func NewAMFHandler(smfAddress string, timeout time.Duration) (*AMFHandler, error) {
	conn, err := grpc.NewClient(smfAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接 SMF: %w", err)
	}
	return &AMFHandler{
		registeredUEs: make(map[string]UERecord),
		smfClient:     smfpb.NewSMFServiceClient(conn),
		conn:          conn,
		callTimeout:   timeout,
	}, nil
}

func NewAMFHandlerWithClient(client smfpb.SMFServiceClient, timeout time.Duration) *AMFHandler {
	return &AMFHandler{registeredUEs: make(map[string]UERecord), smfClient: client, callTimeout: timeout}
}

func (h *AMFHandler) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

func validateRegister(req *amfpb.RegisterRequest) error {
	if req.GetUeId() == "" {
		return status.Error(codes.InvalidArgument, "ue_id 不能为空")
	}
	if req.GetUeType() == "" {
		return status.Error(codes.InvalidArgument, "ue_type 不能为空")
	}
	return nil
}

func (h *AMFHandler) Register(ctx context.Context, req *amfpb.RegisterRequest) (*amfpb.RegisterResponse, error) {
	started := time.Now()
	if err := validateRegister(req); err != nil {
		return nil, err
	}
	if req.SignalPower < -110 || req.Sinr < 0 {
		return &amfpb.RegisterResponse{Success: false, Message: fmt.Sprintf("信道质量不足(signal=%.2f dBm, SINR=%.2f dB)", req.SignalPower, req.Sinr), ProcessingMs: float64(time.Since(started).Microseconds()) / 1000}, nil
	}

	h.mu.RLock()
	existing, exists := h.registeredUEs[req.UeId]
	h.mu.RUnlock()
	if exists && existing.State == "REGISTERED" {
		return responseForRecord(existing, "UE 已注册（幂等返回）", started), nil
	}

	amfUEID := fmt.Sprintf("AMF-%s", req.UeId)
	callCtx, cancel := context.WithTimeout(ctx, h.callTimeout)
	defer cancel()
	smfResp, err := h.smfClient.CreateSession(callCtx, &smfpb.CreateSessionRequest{UeId: req.UeId, AmfUeId: amfUEID, Dnn: "internet"})
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "SMF 建立会话失败: %v", err)
	}
	if !smfResp.Success {
		return &amfpb.RegisterResponse{Success: false, Message: "SMF拒绝建立会话: " + smfResp.Message, ProcessingMs: float64(time.Since(started).Microseconds()) / 1000}, nil
	}

	record := UERecord{UEID: req.UeId, AMFUEID: amfUEID, SessionID: smfResp.SessionId, UEIP: smfResp.UeIp, State: "REGISTERED"}
	h.mu.Lock()
	if current, ok := h.registeredUEs[req.UeId]; ok && current.State == "REGISTERED" {
		record = current
	} else {
		h.registeredUEs[req.UeId] = record
	}
	h.mu.Unlock()
	log.Printf("注册成功 ue_id=%s session_id=%s ue_ip=%s", record.UEID, record.SessionID, record.UEIP)
	return responseForRecord(record, "注册和会话建立成功", started), nil
}

func responseForRecord(record UERecord, message string, started time.Time) *amfpb.RegisterResponse {
	return &amfpb.RegisterResponse{Success: true, AmfUeId: record.AMFUEID, SessionId: record.SessionID, UeIp: record.UEIP, Message: message, ProcessingMs: float64(time.Since(started).Microseconds()) / 1000}
}

func (h *AMFHandler) Deregister(ctx context.Context, req *amfpb.DeregisterRequest) (*amfpb.DeregisterResponse, error) {
	if req.GetUeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "ue_id 不能为空")
	}
	h.mu.RLock()
	record, exists := h.registeredUEs[req.UeId]
	h.mu.RUnlock()
	if !exists {
		return &amfpb.DeregisterResponse{Success: true, Message: "UE 已注销（幂等返回）"}, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, h.callTimeout)
	defer cancel()
	resp, err := h.smfClient.DeleteSession(callCtx, &smfpb.DeleteSessionRequest{SessionId: record.SessionID, UeId: record.UEID})
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "SMF 释放会话失败: %v", err)
	}
	if !resp.Success {
		return &amfpb.DeregisterResponse{Success: false, Message: resp.Message}, nil
	}
	h.mu.Lock()
	delete(h.registeredUEs, req.UeId)
	h.mu.Unlock()
	log.Printf("注销成功 ue_id=%s session_id=%s", record.UEID, record.SessionID)
	return &amfpb.DeregisterResponse{Success: true, Message: "UE、会话和转发规则已释放"}, nil
}

func (h *AMFHandler) GetUE(_ context.Context, req *amfpb.GetUERequest) (*amfpb.GetUEResponse, error) {
	if req.GetUeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "ue_id 不能为空")
	}
	h.mu.RLock()
	record, exists := h.registeredUEs[req.UeId]
	h.mu.RUnlock()
	if !exists {
		return &amfpb.GetUEResponse{Found: false}, nil
	}
	return &amfpb.GetUEResponse{Found: true, UeId: record.UEID, AmfUeId: record.AMFUEID, SessionId: record.SessionID, UeIp: record.UEIP, State: record.State}, nil
}
