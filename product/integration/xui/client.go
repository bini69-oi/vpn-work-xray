package xui

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	// Register modernc SQLite driver for database/sql open calls.
	_ "modernc.org/sqlite"
)

type ClientSpec struct {
	DBPath      string
	InboundPort int
	Email       string
	UUID        string
	Flow        string
	TotalBytes  int64
	ExpiresAt   *time.Time
}

func totalGBFromBytes(totalBytes int64) int64 {
	if totalBytes <= 0 {
		return 0
	}
	gb := totalBytes / (1024 * 1024 * 1024)
	if gb <= 0 {
		return 1
	}
	return gb
}

func UpsertClient(ctx context.Context, spec ClientSpec) error {
	if spec.DBPath == "" {
		return errors.New("x-ui db path is required")
	}
	if spec.InboundPort <= 0 {
		return errors.New("x-ui inbound port is required")
	}
	if spec.Email == "" || spec.UUID == "" {
		return errors.New("email and uuid are required")
	}
	db, err := sql.Open("sqlite", spec.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var inboundID int64
	var settingsRaw string
	if err := tx.QueryRowContext(ctx, `SELECT id, settings FROM inbounds WHERE protocol = ? AND port = ? ORDER BY id DESC LIMIT 1`, "vless", spec.InboundPort).Scan(&inboundID, &settingsRaw); err != nil {
		return fmt.Errorf("find inbound: %w", err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal([]byte(settingsRaw), &settings); err != nil {
		return fmt.Errorf("decode inbound settings: %w", err)
	}
	clientsAny, _ := settings["clients"].([]any)
	flow := spec.Flow
	if flow == "" {
		flow = "xtls-rprx-vision"
	}
	expiryMS := int64(0)
	if spec.ExpiresAt != nil {
		expiryMS = spec.ExpiresAt.UTC().UnixMilli()
	}
	if spec.TotalBytes <= 0 {
		spec.TotalBytes = 1024 * 1024 * 1024 * 1024
	}
	totalGB := totalGBFromBytes(spec.TotalBytes)
	found := false
	for i, item := range clientsAny {
		client, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if email, _ := client["email"].(string); email == spec.Email {
			client["id"] = spec.UUID
			client["flow"] = flow
			client["enable"] = true
			client["expiryTime"] = expiryMS
			client["totalGB"] = totalGB
			clientsAny[i] = client
			found = true
			break
		}
	}
	if !found {
		clientsAny = append(clientsAny, map[string]any{
			"id":         spec.UUID,
			"flow":       flow,
			"email":      spec.Email,
			"enable":     true,
			"expiryTime": expiryMS,
			"totalGB":    totalGB,
			"limitIp":    0,
		})
	}
	settings["clients"] = clientsAny
	updatedSettings, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode inbound settings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE inbounds SET settings = ? WHERE id = ?`, string(updatedSettings), inboundID); err != nil {
		return fmt.Errorf("update inbound settings: %w", err)
	}
	if err := clearInboundLimits(ctx, tx, inboundID); err != nil {
		return err
	}

	var clientTrafficID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM client_traffics WHERE inbound_id = ? AND email = ? ORDER BY id DESC LIMIT 1`, inboundID, spec.Email).Scan(&clientTrafficID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO client_traffics(inbound_id, enable, email, up, down, all_time, expiry_time, total, reset, last_online)
			 VALUES(?, 1, ?, 0, 0, 0, ?, ?, 0, 0)`,
			inboundID,
			spec.Email,
			expiryMS,
			spec.TotalBytes,
		)
		if err != nil {
			return fmt.Errorf("insert client_traffics: %w", err)
		}
	case err != nil:
		return fmt.Errorf("query client_traffics: %w", err)
	default:
		_, err = tx.ExecContext(ctx, `UPDATE client_traffics SET enable = 1, expiry_time = ?, total = ? WHERE id = ?`, expiryMS, spec.TotalBytes, clientTrafficID)
		if err != nil {
			return fmt.Errorf("update client_traffics: %w", err)
		}
	}

	return tx.Commit()
}

type ClientLifecycleSpec struct {
	DBPath      string
	InboundPort int
	Email       string
	Enable      bool
	TotalBytes  int64
	ExpiresAt   *time.Time
}

type ClientUsage struct {
	Enable    bool
	UpBytes   int64
	DownBytes int64
	Total     int64
	ExpiryMS  int64
}

func UpdateClientLifecycle(ctx context.Context, spec ClientLifecycleSpec) error {
	if spec.DBPath == "" {
		return errors.New("x-ui db path is required")
	}
	if spec.InboundPort <= 0 {
		return errors.New("x-ui inbound port is required")
	}
	if spec.Email == "" {
		return errors.New("email is required")
	}
	db, err := sql.Open("sqlite", spec.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var inboundID int64
	var settingsRaw string
	if err := tx.QueryRowContext(ctx, `SELECT id, settings FROM inbounds WHERE protocol = ? AND port = ? ORDER BY id DESC LIMIT 1`, "vless", spec.InboundPort).Scan(&inboundID, &settingsRaw); err != nil {
		return fmt.Errorf("find inbound: %w", err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal([]byte(settingsRaw), &settings); err != nil {
		return fmt.Errorf("decode inbound settings: %w", err)
	}
	expiryMS := int64(0)
	if spec.ExpiresAt != nil {
		expiryMS = spec.ExpiresAt.UTC().UnixMilli()
	}
	if spec.TotalBytes <= 0 {
		spec.TotalBytes = 1024 * 1024 * 1024 * 1024
	}
	clientsAny, _ := settings["clients"].([]any)
	totalGB := totalGBFromBytes(spec.TotalBytes)
	updated := make([]any, 0, len(clientsAny))
	found := false
	for _, item := range clientsAny {
		client, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if email, _ := client["email"].(string); email == spec.Email {
			found = true
			client["enable"] = spec.Enable
			client["expiryTime"] = expiryMS
			client["totalGB"] = totalGB
			updated = append(updated, client)
			continue
		}
		updated = append(updated, item)
	}
	if !found && spec.Enable {
		updated = append(updated, map[string]any{
			"email":      spec.Email,
			"enable":     true,
			"expiryTime": expiryMS,
			"totalGB":    totalGB,
			"limitIp":    0,
		})
	}
	settings["clients"] = updated
	updatedSettings, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode inbound settings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE inbounds SET settings = ? WHERE id = ?`, string(updatedSettings), inboundID); err != nil {
		return fmt.Errorf("update inbound settings: %w", err)
	}
	if err := clearInboundLimits(ctx, tx, inboundID); err != nil {
		return err
	}

	enableInt := 0
	if spec.Enable {
		enableInt = 1
	}
	_, err = tx.ExecContext(ctx, `UPDATE client_traffics SET enable = ?, expiry_time = ?, total = ? WHERE inbound_id = ? AND email = ?`, enableInt, expiryMS, spec.TotalBytes, inboundID, spec.Email)
	if err != nil {
		return fmt.Errorf("update client_traffics: %w", err)
	}
	return tx.Commit()
}

func GetClientUsage(ctx context.Context, dbPath string, inboundPort int, email string) (ClientUsage, error) {
	if dbPath == "" || inboundPort <= 0 || email == "" {
		return ClientUsage{}, errors.New("db path, inbound port and email are required")
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ClientUsage{}, err
	}
	defer func() { _ = db.Close() }()
	var inboundID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM inbounds WHERE protocol = ? AND port = ? ORDER BY id DESC LIMIT 1`, "vless", inboundPort).Scan(&inboundID); err != nil {
		return ClientUsage{}, err
	}
	var (
		enableInt int
		up        int64
		down      int64
		total     int64
		expiryMS  int64
	)
	if err := db.QueryRowContext(
		ctx,
		`SELECT COALESCE(enable,0), COALESCE(up,0), COALESCE(down,0), COALESCE(total,0), COALESCE(expiry_time,0)
		 FROM client_traffics WHERE inbound_id = ? AND email = ? ORDER BY id DESC LIMIT 1`,
		inboundID,
		email,
	).Scan(&enableInt, &up, &down, &total, &expiryMS); err != nil {
		return ClientUsage{}, err
	}
	return ClientUsage{
		Enable:    enableInt == 1,
		UpBytes:   up,
		DownBytes: down,
		Total:     total,
		ExpiryMS:  expiryMS,
	}, nil
}

func clearInboundLimits(ctx context.Context, tx *sql.Tx, inboundID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE inbounds SET expiry_time = 0, total = 0 WHERE id = ?`, inboundID)
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such column") {
		return nil
	}
	return fmt.Errorf("clear inbound limits: %w", err)
}
