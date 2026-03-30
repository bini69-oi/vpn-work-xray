package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/xtls/xray-core/product/domain"
)

func BenchmarkStoreWriteProfiles10000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		store, err := Open(ctx, filepath.Join(b.TempDir(), "bench-write.db"))
		if err != nil {
			b.Fatalf("open: %v", err)
		}

		for n := 0; n < 10000; n++ {
			p := makeProfile(fmt.Sprintf("w-%d", n))
			if err := store.UpsertProfile(ctx, p); err != nil {
				b.Fatalf("upsert %d: %v", n, err)
			}
		}
		_ = store.Close()
	}
}

func BenchmarkStoreReadProfiles10000(b *testing.B) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(b.TempDir(), "bench-read.db"))
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()

	for n := 0; n < 10000; n++ {
		if err := store.UpsertProfile(ctx, makeProfile(fmt.Sprintf("r-%d", n))); err != nil {
			b.Fatalf("upsert seed %d: %v", n, err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := store.ListProfiles(ctx)
		if err != nil {
			b.Fatalf("list: %v", err)
		}
		if len(items) != 10000 {
			b.Fatalf("unexpected count: %d", len(items))
		}
	}
}

func makeProfile(id string) domain.Profile {
	return domain.Profile{
		ID:        id,
		Name:      "bench-" + id,
		Enabled:   true,
		RouteMode: domain.RouteModeSplit,
		Endpoints: []domain.Endpoint{
			{Name: "main", Address: "1.1.1.1", Port: 443, Protocol: domain.ProtocolVLESS, ServerTag: "proxy", UUID: "00000000-0000-0000-0000-000000000001"},
		},
		PreferredID: "main",
		ReconnectPolicy: domain.ReconnectPolicy{
			MaxRetries: 1, BaseBackoff: time.Second, MaxBackoff: 2 * time.Second,
		},
	}
}
