package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	smfpb "github.com/5g-core/proto/smf"
	upfpb "github.com/5g-core/proto/upf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Session struct{ SessionID, UEID, AMFUEID, UEIP, DNN, RuleID, State string }

type SMFHandler struct {
	smfpb.UnimplementedSMFServiceServer
	mu          sync.RWMutex
	sessions    map[string]Session
	byUE        map[string]string
	freeIPs     []string
	upfClient   upfpb.UPFServiceClient
	conn        *grpc.ClientConn
	callTimeout time.Duration
}

func defaultIPPool(size int) []string {
	pool := make([]string, 0, size)
	for i := 1; i <= size; i++ {
		pool = append(pool, fmt.Sprintf("10.0.%d.%d", (i-1)/254, (i-1)%254+1))
	}
	return pool
}

func NewSMFHandler(upfAddress string, timeout time.Duration, poolSize int) (*SMFHandler, error) {
	conn, err := grpc.NewClient(upfAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接 UPF: %w", err)
	}
	return newSMFHandler(upfpb.NewUPFServiceClient(conn), conn, timeout, poolSize), nil
}

func NewSMFHandlerWithClient(client upfpb.UPFServiceClient, timeout time.Duration, poolSize int) *SMFHandler {
	return newSMFHandler(client, nil, timeout, poolSize)
}

func newSMFHandler(client upfpb.UPFServiceClient, conn *grpc.ClientConn, timeout time.Duration, poolSize int) *SMFHandler {
	if poolSize < 1 {
		poolSize = 254
	}
	return &SMFHandler{sessions: make(map[string]Session), byUE: make(map[string]string), freeIPs: defaultIPPool(poolSize), upfClient: client, conn: conn, callTimeout: timeout}
}

func (h *SMFHandler) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

func sessionResponse(s Session, message string) *smfpb.CreateSessionResponse {
	return &smfpb.CreateSessionResponse{Success: true, SessionId: s.SessionID, UeIp: s.UEIP, RuleId: s.RuleID, Message: message}
}

func (h *SMFHandler) CreateSession(ctx context.Context, req *smfpb.CreateSessionRequest) (*smfpb.CreateSessionResponse, error) {
	if req.GetUeId() == "" || req.GetAmfUeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "ue_id 和 amf_ue_id 不能为空")
	}
	h.mu.Lock()
	if id, ok := h.byUE[req.UeId]; ok {
		s := h.sessions[id]
		h.mu.Unlock()
		if s.State == "ACTIVE" {
			return sessionResponse(s, "会话已存在（幂等返回）"), nil
		}
		return nil, status.Error(codes.Aborted, "相同 UE 的会话正在创建")
	}
	if len(h.freeIPs) == 0 {
		h.mu.Unlock()
		return &smfpb.CreateSessionResponse{Success: false, Message: "IP地址池已耗尽"}, nil
	}
	ueIP := h.freeIPs[0]
	h.freeIPs = h.freeIPs[1:]
	sessionID := fmt.Sprintf("SESSION-%s", req.UeId)
	s := Session{SessionID: sessionID, UEID: req.UeId, AMFUEID: req.AmfUeId, UEIP: ueIP, DNN: req.Dnn, State: "CREATING"}
	h.sessions[sessionID] = s
	h.byUE[req.UeId] = sessionID
	h.mu.Unlock()

	callCtx, cancel := context.WithTimeout(ctx, h.callTimeout)
	defer cancel()
	upfResp, err := h.upfClient.CreateRule(callCtx, &upfpb.CreateRuleRequest{SessionId: sessionID, UeId: req.UeId, UeIp: ueIP, Dnn: req.Dnn})
	if err != nil || !upfResp.Success {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		delete(h.byUE, req.UeId)
		h.freeIPs = append(h.freeIPs, ueIP)
		h.mu.Unlock()
		if err != nil {
			return nil, status.Errorf(codes.Unavailable, "UPF 创建规则失败: %v", err)
		}
		return &smfpb.CreateSessionResponse{Success: false, Message: upfResp.Message}, nil
	}
	s.RuleID, s.State = upfResp.RuleId, "ACTIVE"
	h.mu.Lock()
	h.sessions[sessionID] = s
	h.mu.Unlock()
	log.Printf("会话建立成功 ue_id=%s session_id=%s ue_ip=%s rule_id=%s", s.UEID, s.SessionID, s.UEIP, s.RuleID)
	return sessionResponse(s, "会话和转发规则建立成功"), nil
}

func (h *SMFHandler) DeleteSession(ctx context.Context, req *smfpb.DeleteSessionRequest) (*smfpb.DeleteSessionResponse, error) {
	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id 不能为空")
	}
	h.mu.RLock()
	s, exists := h.sessions[req.SessionId]
	h.mu.RUnlock()
	if !exists {
		return &smfpb.DeleteSessionResponse{Success: true, Message: "会话已释放（幂等返回）"}, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, h.callTimeout)
	defer cancel()
	resp, err := h.upfClient.DeleteRule(callCtx, &upfpb.DeleteRuleRequest{RuleId: s.RuleID, SessionId: s.SessionID})
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "UPF 删除规则失败: %v", err)
	}
	if !resp.Success {
		return &smfpb.DeleteSessionResponse{Success: false, Message: resp.Message}, nil
	}
	h.mu.Lock()
	if current, ok := h.sessions[s.SessionID]; ok {
		delete(h.sessions, s.SessionID)
		delete(h.byUE, current.UEID)
		h.freeIPs = append(h.freeIPs, current.UEIP)
	}
	h.mu.Unlock()
	log.Printf("会话释放成功 session_id=%s ue_ip=%s", s.SessionID, s.UEIP)
	return &smfpb.DeleteSessionResponse{Success: true, Message: "会话、IP和UPF规则已释放"}, nil
}

func (h *SMFHandler) GetSession(_ context.Context, req *smfpb.GetSessionRequest) (*smfpb.GetSessionResponse, error) {
	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id 不能为空")
	}
	h.mu.RLock()
	s, exists := h.sessions[req.SessionId]
	h.mu.RUnlock()
	if !exists {
		return &smfpb.GetSessionResponse{Found: false}, nil
	}
	return &smfpb.GetSessionResponse{Found: true, SessionId: s.SessionID, UeId: s.UEID, UeIp: s.UEIP, RuleId: s.RuleID, State: s.State}, nil
}
