package reconnect

import (
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/xtls/xray-core/internal/domain"
)

type Decision struct {
	Retry bool
	Delay time.Duration
	Degraded bool
}

type Engine struct {
	rng *rand.Rand
	mu sync.Mutex
	failures map[string][]time.Time
}

func NewEngine(seed int64) *Engine {
	// #nosec G404 -- pseudo-random jitter is intentional and not used for cryptography.
	return &Engine{
		rng: rand.New(rand.NewSource(seed)),
		failures: map[string][]time.Time{},
	}
}

func (e *Engine) Next(policy domain.ReconnectPolicy, attempt int) Decision {
	return e.next("", policy, attempt)
}

func (e *Engine) NextForProfile(profileID string, policy domain.ReconnectPolicy, attempt int) Decision {
	return e.next(profileID, policy, attempt)
}

func (e *Engine) next(profileID string, policy domain.ReconnectPolicy, attempt int) Decision {
	if attempt >= policy.MaxRetries {
		return Decision{Retry: false}
	}
	exp := float64(policy.BaseBackoff) * math.Pow(2, float64(attempt))
	delay := time.Duration(exp)
	if delay > policy.MaxBackoff {
		delay = policy.MaxBackoff
	}
	// Add 0..20% jitter to reduce synchronized retries.
	jitter := time.Duration(e.rng.Int63n(int64(delay/5 + 1)))
	decision := Decision{
		Retry: true,
		Delay: delay + jitter,
	}
	if strings.TrimSpace(profileID) == "" {
		return decision
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now().UTC()
	window := policy.FailureWindow
	if window <= 0 {
		window = 60 * time.Second
	}
	events := append(e.failures[profileID], now)
	filtered := make([]time.Time, 0, len(events))
	for _, ts := range events {
		if now.Sub(ts) <= window {
			filtered = append(filtered, ts)
		}
	}
	e.failures[profileID] = filtered
	if policy.DegradedFailures > 0 && len(filtered) >= policy.DegradedFailures {
		decision.Degraded = true
	}
	return decision
}
