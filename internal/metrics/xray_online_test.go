package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQueryXrayOnlineUserCount_refused(t *testing.T) {
	_, err := QueryXrayOnlineUserCount(context.Background(), "127.0.0.1:1", 200*time.Millisecond)
	require.Error(t, err)
}
