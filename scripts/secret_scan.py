#!/usr/bin/env python3
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
ALLOWLIST = ROOT / ".secret-scan-allowlist"

PATTERNS = [
    re.compile(r"-----BEGIN (RSA |EC |OPENSSH |)?PRIVATE KEY-----"),
    re.compile(r"AKIA[0-9A-Z]{16}"),
    re.compile(r'(?i)(api[_-]?key|api[_-]?token|secret|password|bearer)\s*[:=]\s*["\']?[a-zA-Z0-9_\-]{20,}'),
    re.compile(r"\b\d{8,10}:[A-Za-z0-9_-]{35}\b"),  # Telegram bot tokens
]

SKIP_DIRS = {".git", "node_modules", "vendor", ".cursor", "var", ".venv", "venv"}
SKIP_SUFFIX = {".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".zip", ".tar", ".gz", ".db", ".dat", ".pb.go", ".proto"}


def read_allowlist() -> list[str]:
    if not ALLOWLIST.exists():
        return []
    return [line.strip() for line in ALLOWLIST.read_text(encoding="utf-8").splitlines() if line.strip() and not line.strip().startswith("#")]


def allowed(path: pathlib.Path, line: str, rules: list[str]) -> bool:
    text = f"{path}:{line}"
    for rule in rules:
        if rule in text:
            return True
    return False


def main() -> int:
    rules = read_allowlist()
    findings: list[str] = []
    for path in ROOT.rglob("*"):
        if not path.is_file():
            continue
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        if path.suffix.lower() in SKIP_SUFFIX:
            continue
        try:
            content = path.read_text(encoding="utf-8")
        except Exception:
            continue
        for idx, line in enumerate(content.splitlines(), start=1):
            for pat in PATTERNS:
                if pat.search(line) and not allowed(path.relative_to(ROOT), line, rules):
                    findings.append(f"{path.relative_to(ROOT)}:{idx}: {line[:180]}")
                    break
    if findings:
        print("[secret-scan] potential secrets found:")
        for item in findings:
            print(item)
        return 1
    print("[secret-scan] no secrets detected")
    return 0


if __name__ == "__main__":
    sys.exit(main())
