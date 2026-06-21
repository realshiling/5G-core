package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	upfpb "github.com/5g-core/proto/upf"
)

// ForwardingRule 转发规则
type ForwardingRule struct {
	RuleID           string
	SessionID        string
	UeID             string
	UeIP             string
	DNN              string
	PacketsForwarded uint64
	CreatedAt        time.Time
	Active           bool
}

type UPFHandler struct {
	upfpb.UnimplementedUPFServiceServer
	rules map[string]*ForwardingRule // rule_id -> rule
	mu    sync.RWMutex               // 并发安全
}

func NewUPFHandler() *UPFHandler {
	return &UPFHandler{
		rules: make(map[string]*ForwardingRule),
	}
}

// CreateRule SMF下发转发规则
func (h *UPFHandler) CreateRule(ctx context.Context, req *upfpb.CreateRuleRequest) (*upfpb.CreateRuleResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("收到转发规则: UE_ID=%s, UE_IP=%s, DNN=%s",
		req.UeId, req.UeIp, req.Dnn)

	ruleID := fmt.Sprintf("RULE-%s-%d", req.UeId, time.Now().UnixNano())

	h.rules[ruleID] = &ForwardingRule{
		RuleID:    ruleID,
		SessionID: req.SessionId,
		UeID:      req.UeId,
		UeIP:      req.UeIp,
		DNN:       req.Dnn,
		CreatedAt: time.Now(),
		Active:    true,
	}

	log.Printf("转发规则创建成功: RULE_ID=%s, UE_IP=%s", ruleID, req.UeIp)

	// 模拟数据包转发
	go h.simulateForwarding(ruleID)

	return &upfpb.CreateRuleResponse{
		Success: true,
		RuleId:  ruleID,
		Message: fmt.Sprintf("转发规则已下发，UE %s 可以通过 %s 上网", req.UeId, req.UeIp),
	}, nil
}

// simulateForwarding 模拟数据包转发，体现低延迟
func (h *UPFHandler) simulateForwarding(ruleID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.Lock()
		rule, exists := h.rules[ruleID]
		if !exists || !rule.Active {
			h.mu.Unlock()
			return
		}
		rule.PacketsForwarded += 10
		h.mu.Unlock()

		log.Printf("UPF 转发中: RULE_ID=%s, UE_IP=%s, 已转发包数=%d",
			ruleID, rule.UeIP, rule.PacketsForwarded)
	}
}

// DeleteRule 删除转发规则
func (h *UPFHandler) DeleteRule(ctx context.Context, req *upfpb.DeleteRuleRequest) (*upfpb.DeleteRuleResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("删除转发规则: RULE_ID=%s", req.RuleId)

	rule, exists := h.rules[req.RuleId]
	if !exists {
		return &upfpb.DeleteRuleResponse{
			Success: false,
			Message: "规则不存在",
		}, nil
	}

	rule.Active = false
	delete(h.rules, req.RuleId)

	log.Printf("转发规则删除成功: RULE_ID=%s", req.RuleId)

	return &upfpb.DeleteRuleResponse{
		Success: true,
		Message: "转发规则已删除",
	}, nil
}

// GetStats 查询转发统计，用于延迟监控
func (h *UPFHandler) GetStats(ctx context.Context, req *upfpb.GetStatsRequest) (*upfpb.GetStatsResponse, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rule, exists := h.rules[req.RuleId]
	if !exists {
		return &upfpb.GetStatsResponse{}, nil
	}

	return &upfpb.GetStatsResponse{
		RuleId:           rule.RuleID,
		UeIp:             rule.UeIP,
		PacketsForwarded: rule.PacketsForwarded,
		Active:           rule.Active,
	}, nil
}
