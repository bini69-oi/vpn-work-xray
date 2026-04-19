package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "vpn-productctl error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var apiAddr = flag.String("api", "http://127.0.0.1:8080", "daemon API URL")
	var apiToken = flag.String("token", "", "API bearer token (or VPN_PRODUCT_API_TOKEN env)")
	flag.Parse()
	token := *apiToken
	if token == "" {
		token = os.Getenv("VPN_PRODUCT_API_TOKEN")
	}

	args := flag.Args()
	if len(args) == 0 {
		usage()
		return nil
	}

	switch args[0] {
	case "status":
		return doGet(*apiAddr+"/v1/status", token)
	case "profiles":
		return doGet(*apiAddr+"/v1/profiles", token)
	case "profile-import":
		if len(args) < 2 {
			return fmt.Errorf("profile-import requires <json-file>")
		}
		raw, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		return doPostRaw(*apiAddr+"/v1/profiles/upsert", raw, token)
	case "profile-delete":
		if len(args) < 2 {
			return fmt.Errorf("profile-delete requires <profile-id>")
		}
		return doPost(*apiAddr+"/v1/profiles/delete", map[string]string{"profileId": args[1]}, token)
	case "quota-set":
		if len(args) < 3 {
			return fmt.Errorf("quota-set requires <profile-id> <limit-mb>")
		}
		var limit int64
		if _, err := fmt.Sscan(args[2], &limit); err != nil {
			return err
		}
		return doPost(*apiAddr+"/v1/quota/set", map[string]any{"profileId": args[1], "limitMb": limit}, token)
	case "quota-add":
		if len(args) < 4 {
			return fmt.Errorf("quota-add requires <profile-id> <upload-bytes> <download-bytes>")
		}
		var up, down int64
		if _, err := fmt.Sscan(args[2], &up); err != nil {
			return err
		}
		if _, err := fmt.Sscan(args[3], &down); err != nil {
			return err
		}
		return doPost(*apiAddr+"/v1/quota/add", map[string]any{"profileId": args[1], "uploadBytes": up, "downloadBytes": down}, token)
	case "quota-block":
		if len(args) < 3 {
			return fmt.Errorf("quota-block requires <profile-id> <true|false>")
		}
		blocked := args[2] == "true"
		return doPost(*apiAddr+"/v1/quota/block", map[string]any{"profileId": args[1], "blocked": blocked}, token)
	case "stats-profiles":
		return doGet(*apiAddr+"/v1/stats/profiles", token)
	case "panel-user-upsert":
		if len(args) < 5 {
			return fmt.Errorf("panel-user-upsert requires <id> <username> <profile-id> <status>")
		}
		return doPost(*apiAddr+"/v1/integration/3xui/users/upsert", map[string]any{
			"id":         args[1],
			"panel":      "3x-ui",
			"externalId": args[1],
			"username":   args[2],
			"profileId":  args[3],
			"status":     args[4],
		}, token)
	case "panel-users":
		return doGet(*apiAddr+"/v1/integration/3xui/users?panel=3x-ui", token)
	case "diagnostics":
		return doGet(*apiAddr+"/v1/diagnostics/snapshot", token)
	case "connect":
		if len(args) < 2 {
			return fmt.Errorf("connect requires <profile-id>")
		}
		return doPost(*apiAddr+"/v1/connect", map[string]string{"profileId": args[1]}, token)
	case "disconnect":
		return doPost(*apiAddr+"/v1/disconnect", map[string]string{}, token)
	default:
		usage()
		return fmt.Errorf("unsupported command: %s", args[0])
	}
}

func usage() {
	fmt.Println("vpn-productctl [--api URL] [--token TOKEN] <status|profiles|profile-import <file>|profile-delete <id>|quota-set <id> <mb>|quota-add <id> <up> <down>|quota-block <id> <true|false>|stats-profiles|panel-user-upsert <id> <username> <profile-id> <status>|panel-users|diagnostics|connect <profile-id>|disconnect>")
}

func doGet(url string, token string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return printBody(resp.Body)
}

func doPost(url string, payload any, token string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return doPostRaw(url, raw, token)
}

func doPostRaw(url string, raw []byte, token string) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return printBody(resp.Body)
}

func printBody(r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}
