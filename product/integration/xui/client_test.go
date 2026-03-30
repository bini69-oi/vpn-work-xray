package xui

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestUpsertClient(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "xui.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	_, err = db.Exec(`
CREATE TABLE inbounds (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 protocol TEXT,
 port INTEGER,
 settings TEXT,
 expiry_time INTEGER DEFAULT 0,
 total INTEGER DEFAULT 0
);
CREATE TABLE client_traffics (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 inbound_id INTEGER,
 enable numeric,
 email TEXT,
 up INTEGER,
 down INTEGER,
 all_time INTEGER,
 expiry_time INTEGER,
 total INTEGER,
 reset INTEGER,
 last_online INTEGER
);`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	settings, _ := json.Marshal(map[string]any{
		"clients": []any{
			map[string]any{"id": "old", "email": "someone"},
		},
	})
	_, err = db.Exec(`INSERT INTO inbounds(protocol, port, settings, expiry_time, total) VALUES(?, ?, ?, ?, ?)`, "vless", 8443, string(settings), 12345, 98765)
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	exp := time.Now().UTC().Add(24 * time.Hour)
	if err := UpsertClient(context.Background(), ClientSpec{
		DBPath:      dbPath,
		InboundPort: 8443,
		Email:       "tg_1",
		UUID:        "11111111-2222-3333-4444-555555555555",
		Flow:        "xtls-rprx-vision",
		TotalBytes:  1024,
		ExpiresAt:   &exp,
	}); err != nil {
		t.Fatalf("upsert client: %v", err)
	}
	var settingsOut string
	if err := db.QueryRow(`SELECT settings FROM inbounds WHERE port = 8443`).Scan(&settingsOut); err != nil {
		t.Fatalf("query settings: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settingsOut), &parsed); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	clients, _ := parsed["clients"].([]any)
	if len(clients) < 2 {
		t.Fatalf("expected appended client, got %d", len(clients))
	}
	var found map[string]any
	for _, item := range clients {
		cm, _ := item.(map[string]any)
		if email, _ := cm["email"].(string); email == "tg_1" {
			found = cm
			break
		}
	}
	if found == nil {
		t.Fatalf("expected tg_1 client in settings")
	}
	if enabled, ok := found["enable"].(bool); !ok || !enabled {
		t.Fatalf("expected settings client enabled=true, got %v", found["enable"])
	}
	if gotGB, ok := found["totalGB"].(float64); !ok || int64(gotGB) != 1 {
		t.Fatalf("expected settings totalGB=1, got %v", found["totalGB"])
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM client_traffics WHERE inbound_id = 1 AND email = ?`, "tg_1").Scan(&count); err != nil {
		t.Fatalf("query client_traffics: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one client_traffics row, got %d", count)
	}
	usage, err := GetClientUsage(context.Background(), dbPath, 8443, "tg_1")
	if err != nil {
		t.Fatalf("get usage: %v", err)
	}
	if !usage.Enable {
		t.Fatalf("expected enabled usage")
	}
	if usage.Total != 1024 {
		t.Fatalf("expected total 1024, got %d", usage.Total)
	}
	var inboundTotal, inboundExpiry int64
	if err := db.QueryRow(`SELECT total, expiry_time FROM inbounds WHERE port = 8443`).Scan(&inboundTotal, &inboundExpiry); err != nil {
		t.Fatalf("query inbound limits: %v", err)
	}
	if inboundTotal != 0 || inboundExpiry != 0 {
		t.Fatalf("expected inbound limits cleared, got total=%d expiry=%d", inboundTotal, inboundExpiry)
	}
}
