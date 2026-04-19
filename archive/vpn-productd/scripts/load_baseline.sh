#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
TOKEN="${TOKEN:-}"
OUT_DIR="${OUT_DIR:-benchmarks/load}"
SCENARIO="${SCENARIO:-steady}"
REQUESTS="${REQUESTS:-500}"

if [[ -z "${TOKEN}" ]]; then
  echo "TOKEN is required"
  exit 1
fi

mkdir -p "${OUT_DIR}"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
outfile="${OUT_DIR}/${SCENARIO}-${stamp}.json"

tmp_codes="$(mktemp)"
trap 'rm -f "${tmp_codes}"' EXIT

start="$(date +%s)"
for ((i=1; i<=REQUESTS; i++)); do
  code="$(
    curl -sS -o /dev/null -w "%{http_code}" \
      -H "Authorization: Bearer ${TOKEN}" \
      "${BASE_URL}/v1/status"
  )"
  echo "${code}" >> "${tmp_codes}"
done
end="$(date +%s)"

total="$(wc -l < "${tmp_codes}" | tr -d ' ')"
errors="$(awk '$1 >= 500 {c++} END {print c+0}' "${tmp_codes}")"
elapsed="$((end-start))"
if [[ "${elapsed}" -le 0 ]]; then elapsed=1; fi
rps="$((total/elapsed))"

python3 - "${outfile}" "${SCENARIO}" "${total}" "${errors}" "${elapsed}" "${rps}" <<'PY'
import json, sys, time
outfile, scenario, total, errors, elapsed, rps = sys.argv[1:]
total = int(total)
errors = int(errors)
elapsed = int(elapsed)
rps = int(rps)
result = {
    "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "scenario": scenario,
    "total_requests": total,
    "errors_5xx": errors,
    "error_rate": (errors / total) if total else 0.0,
    "elapsed_seconds": elapsed,
    "rps": rps,
    "note": "Use Prometheus histogram for precise p95/p99 comparison."
}
with open(outfile, "w", encoding="utf-8") as f:
    json.dump(result, f, indent=2)
print(outfile)
PY

echo "load baseline saved to ${outfile}"
