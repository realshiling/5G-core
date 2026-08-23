package main

import (
	"math/rand"
	"testing"
)

func TestStableAndDegradingScenarios(t *testing.T) {
	stable := sampleChannel("stable", rand.New(rand.NewSource(42)))
	bad := sampleChannel("degrading", rand.New(rand.NewSource(42)))
	if stable.SignalPower <= -110 || stable.SINR <= 0 {
		t.Fatalf("stable sample should pass: %+v", stable)
	}
	if bad.SignalPower >= stable.SignalPower {
		t.Fatalf("degrading should be weaker: stable=%+v bad=%+v", stable, bad)
	}
}
