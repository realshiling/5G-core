package main

import (
	"context"
	"testing"
	"time"

	upfpb "github.com/5g-core/proto/upf"
)

func TestRuleLifecycleAndIdempotency(t *testing.T) {
	h := NewUPFHandlerWithInterval(time.Millisecond)
	req := &upfpb.CreateRuleRequest{SessionId: "s1", UeId: "u1", UeIp: "10.0.0.1", Dnn: "internet"}
	first, err := h.CreateRule(context.Background(), req)
	if err != nil || !first.Success {
		t.Fatalf("create: resp=%v err=%v", first, err)
	}
	second, err := h.CreateRule(context.Background(), req)
	if err != nil || second.RuleId != first.RuleId {
		t.Fatalf("idempotent create: resp=%v err=%v", second, err)
	}
	time.Sleep(3 * time.Millisecond)
	stats, err := h.GetStats(context.Background(), &upfpb.GetStatsRequest{SessionId: "s1"})
	if err != nil || !stats.Active || stats.PacketsForwarded == 0 {
		t.Fatalf("stats: resp=%v err=%v", stats, err)
	}
	deleted, err := h.DeleteRule(context.Background(), &upfpb.DeleteRuleRequest{SessionId: "s1"})
	if err != nil || !deleted.Success {
		t.Fatalf("delete: resp=%v err=%v", deleted, err)
	}
	deleted, err = h.DeleteRule(context.Background(), &upfpb.DeleteRuleRequest{SessionId: "s1"})
	if err != nil || !deleted.Success {
		t.Fatalf("idempotent delete: resp=%v err=%v", deleted, err)
	}
}
