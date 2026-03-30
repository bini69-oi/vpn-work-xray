package reconnect

import (
	"math"
	"math/rand"
	"time"

	"github.com/xtls/xray-core/product/domain"
)

type Decision struct {
	Retry bool
	Delay time.Duration
}

type Engine struct {
	rng *rand.Rand
}

func NewEngine(seed int64) *Engine {
	// #nosec G404 -- pseudo-random jitter is intentional and not used for cryptography.
	return &Engine{rng: rand.New(rand.NewSource(seed))}
}

func (e *Engine) Next(policy domain.ReconnectPolicy, attempt int) Decision {
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
	return Decision{
		Retry: true,
		Delay: delay + jitter,
	}
}
