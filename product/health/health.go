package health

import (
	"context"
	"time"
)

type ProbeResult struct {
	Healthy      bool          `json:"healthy"`
	Latency      time.Duration `json:"latency"`
	ErrorMessage string        `json:"errorMessage,omitempty"`
	CheckedAt    time.Time     `json:"checkedAt"`
}

type Prober interface {
	Probe(ctx context.Context) ProbeResult
}

type StaticProber struct {
	Default ProbeResult
}

func (p StaticProber) Probe(_ context.Context) ProbeResult {
	out := p.Default
	out.CheckedAt = time.Now().UTC()
	return out
}
