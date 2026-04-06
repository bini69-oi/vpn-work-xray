package metrics

import (
	"context"
	"time"

	statscommand "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// QueryXrayOnlineUserCount calls Xray StatsService.GetAllOnlineUsers over gRPC (same API as `xray api statsgetallonlineusers`).
// addr is host:port of the Xray API inbound (e.g. 127.0.0.1:10085). Returns the number of online user entries.
func QueryXrayOnlineUserCount(ctx context.Context, addr string, dialTimeout time.Duration) (int64, error) {
	if dialTimeout <= 0 {
		dialTimeout = 3 * time.Second
	}
	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	client := statscommand.NewStatsServiceClient(conn)
	resp, err := client.GetAllOnlineUsers(dctx, &statscommand.GetAllOnlineUsersRequest{})
	if err != nil {
		return 0, err
	}
	return int64(len(resp.GetUsers())), nil
}
