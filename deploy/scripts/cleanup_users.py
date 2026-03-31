#!/usr/bin/env python3
import json
import os
import sqlite3

PDB = "/var/lib/vpn-product/product.db"
XDB = "/etc/x-ui/x-ui.db"
DEFAULT_KEEP_USERS = {"Максим", "owner_main"}


def parse_keep_users() -> set[str]:
    raw = os.getenv("KEEP_USERS", "")
    if not raw.strip():
        return set(DEFAULT_KEEP_USERS)
    users = {item.strip() for item in raw.split(",") if item.strip()}
    return users if users else set(DEFAULT_KEEP_USERS)


KEEP_USERS = parse_keep_users()


def cleanup_product_db() -> None:
    conn = sqlite3.connect(PDB)
    cur = conn.cursor()

    # Revoke subscriptions not in keep list.
    cur.execute(
        "UPDATE subscriptions SET revoked = 1, revoked_at = datetime('now'), status = 'revoked', updated_at = datetime('now') WHERE user_id NOT IN ({})".format(
            ",".join("?" * len(KEEP_USERS))
        ),
        tuple(KEEP_USERS),
    )

    # Delete panel mappings not in keep list.
    cur.execute(
        "DELETE FROM panel_users WHERE external_id NOT IN ({})".format(
            ",".join("?" * len(KEEP_USERS))
        ),
        tuple(KEEP_USERS),
    )

    # Keep only base profile plus profiles referenced by active kept subscriptions.
    keep_profiles = {"xui-test-vpn"}
    rows = cur.execute(
        "SELECT profile_ids_json FROM subscriptions WHERE revoked = 0 AND user_id IN ({})".format(
            ",".join("?" * len(KEEP_USERS))
        ),
        tuple(KEEP_USERS),
    ).fetchall()
    for (profile_ids_json,) in rows:
        try:
            pids = json.loads(profile_ids_json or "[]")
        except Exception:
            pids = []
        for pid in pids:
            if isinstance(pid, str) and pid.strip():
                keep_profiles.add(pid.strip())

    cur.execute("SELECT id FROM profiles")
    all_profiles = [row[0] for row in cur.fetchall()]
    for pid in all_profiles:
        if pid in keep_profiles:
            continue
        cur.execute("DELETE FROM profiles WHERE id = ?", (pid,))
        cur.execute("DELETE FROM profile_quota WHERE profile_id = ?", (pid,))

    conn.commit()
    conn.close()


def cleanup_xui_db() -> None:
    conn = sqlite3.connect(XDB)
    cur = conn.cursor()
    inbound = cur.execute(
        "SELECT id, settings FROM inbounds WHERE protocol='vless' AND port=8443 ORDER BY id DESC LIMIT 1"
    ).fetchone()
    if not inbound:
        conn.close()
        return

    inbound_id, settings_raw = inbound
    settings = json.loads(settings_raw or "{}")
    clients = settings.get("clients", [])
    clients = [c for c in clients if str(c.get("email", "")).strip() in KEEP_USERS]
    settings["clients"] = clients
    cur.execute(
        "UPDATE inbounds SET settings=? WHERE id=?",
        (json.dumps(settings, separators=(",", ":")), inbound_id),
    )

    cur.execute(
        "DELETE FROM client_traffics WHERE inbound_id = ? AND email NOT IN ({})".format(
            ",".join("?" * len(KEEP_USERS))
        ),
        (inbound_id, *tuple(KEEP_USERS)),
    )
    conn.commit()
    conn.close()


def main() -> None:
    cleanup_product_db()
    cleanup_xui_db()
    print("KEPT_USERS:", ",".join(sorted(KEEP_USERS)))


if __name__ == "__main__":
    main()
