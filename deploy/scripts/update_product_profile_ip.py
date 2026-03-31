#!/usr/bin/env python3
import json
import sqlite3
import sys

DB = "/var/lib/vpn-product/product.db"


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit("usage: update_product_profile_ip.py <new_ip>")
    new_ip = sys.argv[1].strip()
    conn = sqlite3.connect(DB)
    cur = conn.cursor()
    changed = 0
    for pid, pj in cur.execute("SELECT id, profile_json FROM profiles").fetchall():
        try:
            d = json.loads(pj)
        except Exception:
            continue
        updated = False
        for ep in d.get("endpoints", []):
            if ep.get("address") != new_ip:
                ep["address"] = new_ip
                updated = True
        if updated:
            cur.execute(
                "UPDATE profiles SET profile_json=?, updated_at=datetime('now') WHERE id=?",
                (json.dumps(d, separators=(",", ":")), pid),
            )
            changed += 1
    conn.commit()
    conn.close()
    print(f"updated_profiles={changed}")


if __name__ == "__main__":
    main()
