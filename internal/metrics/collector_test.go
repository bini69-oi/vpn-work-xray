package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePathForMetrics(t *testing.T) {
	assert.Equal(t, "/public/subscriptions/:token", NormalizePathForMetrics("/public/subscriptions/abc123"))
	assert.Equal(t, "/s/:token", NormalizePathForMetrics("/s/tok"))
	assert.Equal(t, "/v1/status", NormalizePathForMetrics("/v1/status"))
}
