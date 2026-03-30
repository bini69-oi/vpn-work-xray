#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${DB_PATH:-/etc/x-ui/x-ui.db}"
CFG_PATH="${CFG_PATH:-/usr/local/x-ui/bin/config.json}"
INBOUND_PORT="${INBOUND_PORT:-8443}"

python3 - "$DB_PATH" "$CFG_PATH" "$INBOUND_PORT" <<'PY'
import hashlib
import json
import pathlib
import sqlite3
import subprocess
import sys

db_path = pathlib.Path(sys.argv[1])
cfg_path = pathlib.Path(sys.argv[2])
port = int(sys.argv[3])

if not db_path.exists() or not cfg_path.exists():
    raise SystemExit(0)

conn = sqlite3.connect(str(db_path))
cur = conn.cursor()
row = cur.execute(
    "SELECT settings FROM inbounds WHERE protocol='vless' AND port=? ORDER BY id DESC LIMIT 1",
    (port,),
).fetchone()
conn.close()
if not row:
    raise SystemExit(0)

db_clients = []
for c in json.loads(row[0] or "{}").get("clients", []):
    db_clients.append(
        (
            str(c.get("email", "")),
            str(c.get("id", "")),
            str(c.get("flow", "")),
        )
    )
db_clients.sort()
db_sig = hashlib.sha256(repr(db_clients).encode()).hexdigest()

cfg = json.loads(cfg_path.read_text(encoding="utf-8"))
rt_clients = []
for ib in cfg.get("inbounds", []):
    if ib.get("protocol") == "vless" and int(ib.get("port", 0)) == port:
        for c in ib.get("settings", {}).get("clients", []):
            rt_clients.append(
                (
                    str(c.get("email", "")),
                    str(c.get("id", "")),
                    str(c.get("flow", "")),
                )
            )
rt_clients.sort()
rt_sig = hashlib.sha256(repr(rt_clients).encode()).hexdigest()

if db_sig != rt_sig:
    subprocess.run(["systemctl", "restart", "x-ui"], check=False)
PY
