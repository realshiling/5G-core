package main

import (
	"context"
	"testing"
	"time"

	amfpb "github.com/5g-core/proto/amf"
	smfpb "github.com/5g-core/proto/smf"
	"google.golang.org/grpc"
)

type fakeSMF struct{ created, deleted int }

func (f *fakeSMF) CreateSession(_ context.Context, in *smfpb.CreateSessionRequest, _ ...grpc.CallOption) (*smfpb.CreateSessionResponse, error) {
	f.created++
	return &smfpb.CreateSessionResponse{Success: true, SessionId: "session-" + in.UeId, UeIp: "10.0.0.1", RuleId: "rule-1"}, nil
}
func (f *fakeSMF) DeleteSession(context.Context, *smfpb.DeleteSessionRequest, ...grpc.CallOption) (*smfpb.DeleteSessionResponse, error) {
	f.deleted++
	return &smfpb.DeleteSessionResponse{Success: true}, nil
}
func (f *fakeSMF) GetSession(context.Context, *smfpb.GetSessionRequest, ...grpc.CallOption) (*smfpb.GetSessionResponse, error) {
	return &smfpb.GetSessionResponse{}, nil
}

func TestRegisterChannelGateAndLifecycle(t *testing.T) {
	fake := &fakeSMF{}
	h := NewAMFHandlerWithClient(fake, time.Second)
	bad, err := h.Register(context.Background(), &amfpb.RegisterRequest{UeId: "bad", UeType: "sim", SignalPower: -120, Sinr: 10})
	if err != nil || bad.Success || fake.created != 0 {
		t.Fatalf("bad channel accepted: %v %v", bad, err)
	}
	req := &amfpb.RegisterRequest{UeId: "u1", UeType: "sim", SignalPower: -80, Sinr: 20}
	first, err := h.Register(context.Background(), req)
	if err != nil || !first.Success {
		t.Fatalf("register: %v %v", first, err)
	}
	again, err := h.Register(context.Background(), req)
	if err != nil || again.SessionId != first.SessionId || fake.created != 1 {
		t.Fatalf("idempotency failed: %v %v calls=%d", again, err, fake.created)
	}
	found, err := h.GetUE(context.Background(), &amfpb.GetUERequest{UeId: "u1"})
	if err != nil || !found.Found {
		t.Fatalf("get: %v %v", found, err)
	}
	deleted, err := h.Deregister(context.Background(), &amfpb.DeregisterRequest{UeId: "u1"})
	if err != nil || !deleted.Success || fake.deleted != 1 {
		t.Fatalf("delete: %v %v", deleted, err)
	}
	deleted, err = h.Deregister(context.Background(), &amfpb.DeregisterRequest{UeId: "u1"})
	if err != nil || !deleted.Success || fake.deleted != 1 {
		t.Fatalf("idempotent delete: %v %v", deleted, err)
	}
}
