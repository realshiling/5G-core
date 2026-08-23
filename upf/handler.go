package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	upfpb "github.com/5g-core/proto/upf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ForwardingRule struct {
	RuleID, SessionID, UEID, UEIP, DNN string
	PacketsForwarded                   uint64
	CreatedAt                          time.Time
	Active                             bool
	cancel                             context.CancelFunc
}

type UPFHandler struct {
	upfpb.UnimplementedUPFServiceServer
	mu             sync.RWMutex
	rules          map[string]*ForwardingRule
	bySession      map[string]string
	packetInterval time.Duration
}

func NewUPFHandler() *UPFHandler { return NewUPFHandlerWithInterval(time.Second) }
func NewUPFHandlerWithInterval(interval time.Duration) *UPFHandler {
	return &UPFHandler{rules: make(map[string]*ForwardingRule), bySession: make(map[string]string), packetInterval: interval}
}

func (h *UPFHandler) CreateRule(_ context.Context, req *upfpb.CreateRuleRequest) (*upfpb.CreateRuleResponse, error) {
	if req.GetSessionId() == "" || req.GetUeId() == "" || req.GetUeIp() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id、ue_id 和 ue_ip 不能为空")
	}
	h.mu.Lock()
	if existingID, ok := h.bySession[req.SessionId]; ok {
		rule := h.rules[existingID]
		h.mu.Unlock()
		return &upfpb.CreateRuleResponse{Success: true, RuleId: rule.RuleID, Message: "转发规则已存在（幂等返回）"}, nil
	}
	ruleID := fmt.Sprintf("RULE-%s", req.SessionId)
	workerCtx, cancel := context.WithCancel(context.Background())
	rule := &ForwardingRule{RuleID: ruleID, SessionID: req.SessionId, UEID: req.UeId, UEIP: req.UeIp, DNN: req.Dnn, CreatedAt: time.Now(), Active: true, cancel: cancel}
	h.rules[ruleID] = rule
	h.bySession[req.SessionId] = ruleID
	h.mu.Unlock()
	go h.simulateForwarding(workerCtx, ruleID)
	log.Printf("转发规则创建成功 rule_id=%s session_id=%s ue_ip=%s", ruleID, req.SessionId, req.UeIp)
	return &upfpb.CreateRuleResponse{Success: true, RuleId: ruleID, Message: "转发规则已下发"}, nil
}

func (h *UPFHandler) simulateForwarding(ctx context.Context, ruleID string) {
	ticker := time.NewTicker(h.packetInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.mu.Lock()
			rule, exists := h.rules[ruleID]
			if !exists || !rule.Active {
				h.mu.Unlock()
				return
			}
			rule.PacketsForwarded += 10
			count, ip := rule.PacketsForwarded, rule.UEIP
			h.mu.Unlock()
			log.Printf("模拟转发 rule_id=%s ue_ip=%s packets=%d", ruleID, ip, count)
		}
	}
}

func (h *UPFHandler) DeleteRule(_ context.Context, req *upfpb.DeleteRuleRequest) (*upfpb.DeleteRuleResponse, error) {
	if req.GetRuleId() == "" && req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id 或 session_id 至少提供一个")
	}
	h.mu.Lock()
	ruleID := req.RuleId
	if ruleID == "" {
		ruleID = h.bySession[req.SessionId]
	}
	rule, exists := h.rules[ruleID]
	if !exists {
		h.mu.Unlock()
		return &upfpb.DeleteRuleResponse{Success: true, Message: "规则已删除（幂等返回）"}, nil
	}
	rule.Active = false
	delete(h.rules, ruleID)
	delete(h.bySession, rule.SessionID)
	cancel := rule.cancel
	h.mu.Unlock()
	cancel()
	log.Printf("转发规则删除成功 rule_id=%s", ruleID)
	return &upfpb.DeleteRuleResponse{Success: true, Message: "转发规则已删除"}, nil
}

func (h *UPFHandler) GetStats(_ context.Context, req *upfpb.GetStatsRequest) (*upfpb.GetStatsResponse, error) {
	if req.GetRuleId() == "" && req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id 或 session_id 至少提供一个")
	}
	h.mu.RLock()
	ruleID := req.RuleId
	if ruleID == "" {
		ruleID = h.bySession[req.SessionId]
	}
	rule, exists := h.rules[ruleID]
	if !exists {
		h.mu.RUnlock()
		return &upfpb.GetStatsResponse{}, nil
	}
	resp := &upfpb.GetStatsResponse{RuleId: rule.RuleID, SessionId: rule.SessionID, UeId: rule.UEID, UeIp: rule.UEIP, PacketsForwarded: rule.PacketsForwarded, Active: rule.Active}
	h.mu.RUnlock()
	return resp, nil
}
