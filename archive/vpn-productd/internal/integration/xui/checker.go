package xui

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	// Register modernc SQLite driver for database/sql open calls.
	_ "modernc.org/sqlite"
)

type Checker struct {
	DBPath      string
	InboundPort int
}

func NewChecker(dbPath string, inboundPort int) *Checker {
	return &Checker{DBPath: dbPath, InboundPort: inboundPort}
}

func (c *Checker) Check(ctx context.Context) (bool, map[string]any) {
	if c == nil || c.DBPath == "" || c.InboundPort <= 0 {
		info := map[string]any{
			"ok":        false,
			"checkedAt": time.Now().UTC(),
			"error":     "x-ui checker is not configured",
		}
		return false, info
	}
	info := map[string]any{
		"ok":          false,
		"dbPath":      c.DBPath,
		"inboundPort": c.InboundPort,
		"checkedAt":   time.Now().UTC(),
	}
	db, err := sql.Open("sqlite", c.DBPath)
	if err != nil {
		info["error"] = err.Error()
		return false, info
	}
	defer func() { _ = db.Close() }()

	var inboundID int64
	var settingsRaw string
	err = db.QueryRowContext(ctx, `SELECT id, settings FROM inbounds WHERE protocol = ? AND port = ? ORDER BY id DESC LIMIT 1`, "vless", c.InboundPort).Scan(&inboundID, &settingsRaw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			info["error"] = "inbound not found"
			info["inboundFound"] = false
			return false, info
		}
		info["error"] = fmt.Sprintf("query inbound: %v", err)
		return false, info
	}
	info["inboundFound"] = true
	info["inboundId"] = inboundID

	settings := map[string]any{}
	if err := json.Unmarshal([]byte(settingsRaw), &settings); err != nil {
		info["error"] = fmt.Sprintf("decode inbound settings: %v", err)
		return false, info
	}
	clientsAny, _ := settings["clients"].([]any)
	info["clientCount"] = len(clientsAny)
	if len(clientsAny) <= 0 {
		info["error"] = "no clients in inbound settings"
		return false, info
	}
	info["ok"] = true
	return true, info
}

