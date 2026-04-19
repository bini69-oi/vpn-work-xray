package reconnect

import (
	"testing"
	"time"

	"github.com/xtls/xray-core/internal/domain"
)

func TestEngineNext(t *testing.T) {
	engine := NewEngine(1)
	policy := domain.ReconnectPolicy{
		MaxRetries:  2,
		BaseBackoff: time.Second,
		MaxBackoff:  3 * time.Second,
	}

	first := engine.Next(policy, 0)
	if !first.Retry {
		t.Fatal("first attempt should retry")
	}
	if first.Delay < time.Second {
		t.Fatalf("delay must be >= base backoff, got %v", first.Delay)
	}

	last := engine.Next(policy, 2)
	if last.Retry {
		t.Fatal("attempt at max retries should stop")
	}
}
