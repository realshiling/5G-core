package main

import (
	"math/rand"
	"strings"
)

type ChannelSample struct{ SignalPower, SINR float32 }

func sampleChannel(scenario string, rng *rand.Rand) ChannelSample {
	switch strings.ToLower(scenario) {
	case "stable":
		return ChannelSample{SignalPower: float32(-82 + rng.NormFloat64()*2), SINR: float32(18 + rng.NormFloat64()*1.5)}
	case "edge":
		return ChannelSample{SignalPower: float32(-109 + rng.NormFloat64()*4), SINR: float32(2 + rng.NormFloat64()*2)}
	case "degrading":
		return ChannelSample{SignalPower: float32(-116 + rng.NormFloat64()*3), SINR: float32(-1 + rng.NormFloat64()*2)}
	default: // mixed: 70%稳定、20%边缘、10%恶化
		value := rng.Float64()
		if value < .7 {
			return sampleChannel("stable", rng)
		}
		if value < .9 {
			return sampleChannel("edge", rng)
		}
		return sampleChannel("degrading", rng)
	}
}
