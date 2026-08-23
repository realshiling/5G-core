package main

import (
	"context"
	"testing"
	"time"

	smfpb "github.com/5g-core/proto/smf"
	upfpb "github.com/5g-core/proto/upf"
	"google.golang.org/grpc"
)

type fakeUPF struct{ created, deleted int }

func (f *fakeUPF) CreateRule(_ context.Context, in *upfpb.CreateRuleRequest, _ ...grpc.CallOption) (*upfpb.CreateRuleResponse, error) {
	f.created++
	return &upfpb.CreateRuleResponse{Success: true, RuleId: "rule-" + in.SessionId}, nil
}
func (f *fakeUPF) DeleteRule(_ context.Context, _ *upfpb.DeleteRuleRequest, _ ...grpc.CallOption) (*upfpb.DeleteRuleResponse, error) {
	f.deleted++
	return &upfpb.DeleteRuleResponse{Success: true}, nil
}
func (f *fakeUPF) GetStats(context.Context, *upfpb.GetStatsRequest, ...grpc.CallOption) (*upfpb.GetStatsResponse, error) {
	return &upfpb.GetStatsResponse{}, nil
}

func TestSessionLifecycleReusesIP(t *testing.T) {
	fake := &fakeUPF{}
	h := NewSMFHandlerWithClient(fake, time.Second, 1)
	req := createRequest("u1")
	first, err := h.CreateSession(context.Background(), req)
	if err != nil || !first.Success {
		t.Fatalf("create: %v %v", first, err)
	}
	again, err := h.CreateSession(context.Background(), req)
	if err != nil || again.SessionId != first.SessionId || fake.created != 1 {
		t.Fatalf("idempotency failed: %v %v calls=%d", again, err, fake.created)
	}
	exhausted, err := h.CreateSession(context.Background(), createRequest("u2"))
	if err != nil || exhausted.Success {
		t.Fatalf("expected pool exhaustion: %v %v", exhausted, err)
	}
	deleted, err := h.DeleteSession(context.Background(), deleteRequest(first.SessionId, "u1"))
	if err != nil || !deleted.Success {
		t.Fatalf("delete: %v %v", deleted, err)
	}
	second, err := h.CreateSession(context.Background(), createRequest("u2"))
	if err != nil || !second.Success || second.UeIp != first.UeIp {
		t.Fatalf("IP not reused: %v %v", second, err)
	}
}

func createRequest(id string) *smfpb.CreateSessionRequest {
	return &smfpb.CreateSessionRequest{UeId: id, AmfUeId: "amf-" + id, Dnn: "internet"}
}
func deleteRequest(session, id string) *smfpb.DeleteSessionRequest {
	return &smfpb.DeleteSessionRequest{SessionId: session, UeId: id}
}
