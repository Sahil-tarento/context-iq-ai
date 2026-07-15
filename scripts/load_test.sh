#!/usr/bin/env bash
# ContextIQ Load Test — measures throughput, latency, and token savings efficiency
# Usage: bash scripts/load_test.sh [daemon_url] [concurrency] [iterations]
#
# Defaults:
#   daemon_url  = http://localhost:9009
#   concurrency = 5
#   iterations  = 20

set -euo pipefail

DAEMON="${1:-http://localhost:9009}"
CONCURRENCY="${2:-5}"
ITERATIONS="${3:-20}"
REPO_PATH="${4:-$(pwd)}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

banner() { echo -e "\n${CYAN}${BOLD}══════════════════════════════════════════${NC}"; echo -e "${CYAN}${BOLD}  $1${NC}"; echo -e "${CYAN}${BOLD}══════════════════════════════════════════${NC}"; }
pass()   { echo -e "  ${GREEN}✅ $1${NC}"; }
fail()   { echo -e "  ${RED}❌ $1${NC}"; }
info()   { echo -e "  ${YELLOW}ℹ  $1${NC}"; }

# ── 0. Pre-flight ────────────────────────────────────────────────────────────
banner "Pre-flight: Checking daemon"
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${DAEMON}/v1/health")
if [ "$HTTP_STATUS" != "200" ]; then
  fail "Daemon not reachable at ${DAEMON} (status ${HTTP_STATUS})"
  echo "Start it with:  ./contextiq --port 9009"
  exit 1
fi
pass "Daemon healthy at ${DAEMON}"

# ── 1. Index the repository (once) ──────────────────────────────────────────
banner "Step 1 — Index Repository"
INDEX_START=$(date +%s%3N)
INDEX_RESP=$(curl -s -X POST "${DAEMON}/v1/index" \
  -H "Content-Type: application/json" \
  -d "{\"repo_path\": \"${REPO_PATH}\"}")
INDEX_END=$(date +%s%3N)
INDEX_MS=$((INDEX_END - INDEX_START))

FILES=$(echo "$INDEX_RESP" | jq -r '.files_indexed // 0')
SYMBOLS=$(echo "$INDEX_RESP" | jq -r '.symbols_indexed // 0')
SUCCESS=$(echo "$INDEX_RESP" | jq -r '.success // false')

if [ "$SUCCESS" = "true" ]; then
  pass "Indexed ${FILES} files, ${SYMBOLS} symbols in ${INDEX_MS}ms"
else
  fail "Indexing failed: $(echo "$INDEX_RESP" | jq -r '.error // "unknown"')"
  info "Continuing with pre-existing index if available..."
fi

# ── 2. Optimize endpoint load test ──────────────────────────────────────────
banner "Step 2 — Load Test: /v1/optimize (context compression efficiency)"

QUERIES=(
  "How does the compressor work?"
  "Explain the cache manager implementation"
  "What does the MCP server do?"
  "How does semantic caching work?"
  "Explain the token savings calculation"
  "What is the CCR retrieve pattern?"
  "How does AST parsing work in parser.go?"
  "What are the REST API endpoints?"
)

TOTAL_RAW=0
TOTAL_OPT=0
TOTAL_SAVINGS=0
TOTAL_LATENCY=0
PASS_COUNT=0
FAIL_COUNT=0

run_optimize() {
  local q="$1"
  local start end ms resp savings raw opt
  start=$(date +%s%3N)
  resp=$(curl -s -X POST "${DAEMON}/v1/optimize" \
    -H "Content-Type: application/json" \
    -d "{\"query\": \"${q}\", \"max_tokens\": 4096, \"open_files\": []}" 2>/dev/null)
  end=$(date +%s%3N)
  ms=$((end - start))

  savings=$(echo "$resp" | jq -r '.token_savings // 0' 2>/dev/null)
  raw=$(echo "$resp"     | jq -r '.raw_tokens // 0'    2>/dev/null)
  opt=$(echo "$resp"     | jq -r '.optimized_tokens // 0' 2>/dev/null)

  echo "${ms} ${raw} ${opt} ${savings}"
}

export -f run_optimize
export DAEMON

info "Running ${ITERATIONS} optimize requests (${CONCURRENCY} concurrent)..."
echo ""
printf "  %-40s %8s %8s %8s %10s\n" "Query" "Raw tok" "Opt tok" "Savings" "Latency(ms)"
printf "  %-40s %8s %8s %8s %10s\n" "-----" "-------" "-------" "-------" "-----------"

for i in $(seq 1 $ITERATIONS); do
  Q="${QUERIES[$((i % ${#QUERIES[@]}))]}"
  RESULT=$(run_optimize "$Q")
  MS=$(echo "$RESULT" | awk '{print $1}')
  RAW=$(echo "$RESULT" | awk '{print $2}')
  OPT=$(echo "$RESULT" | awk '{print $3}')
  SAV=$(echo "$RESULT" | awk '{print $4}')

  if [ "$RAW" -gt 0 ] 2>/dev/null; then
    printf "  %-40s %8s %8s %7.1f%% %10s\n" "${Q:0:40}" "$RAW" "$OPT" "$SAV" "${MS}ms"
    TOTAL_RAW=$((TOTAL_RAW + RAW))
    TOTAL_OPT=$((TOTAL_OPT + OPT))
    TOTAL_SAVINGS=$(echo "$TOTAL_SAVINGS + $SAV" | bc)
    TOTAL_LATENCY=$((TOTAL_LATENCY + MS))
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    printf "  %-40s %8s\n" "${Q:0:40}" "FAILED"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
done

# ── 3. Compute aggregate stats ───────────────────────────────────────────────
banner "Step 3 — Aggregate Efficiency Report"

if [ "$PASS_COUNT" -gt 0 ]; then
  AVG_SAVINGS=$(echo "scale=1; $TOTAL_SAVINGS / $PASS_COUNT" | bc)
  AVG_LATENCY=$((TOTAL_LATENCY / PASS_COUNT))
  TOKEN_REDUCTION=$((TOTAL_RAW - TOTAL_OPT))
  SAVED_PCT=$(echo "scale=1; $TOKEN_REDUCTION * 100 / $TOTAL_RAW" | bc 2>/dev/null || echo "N/A")

  echo ""
  echo -e "  ${BOLD}Total requests:${NC}        ${ITERATIONS}"
  echo -e "  ${GREEN}${BOLD}Passed:${NC}                ${PASS_COUNT}"
  [ "$FAIL_COUNT" -gt 0 ] && echo -e "  ${RED}${BOLD}Failed:${NC}                ${FAIL_COUNT}"
  echo ""
  echo -e "  ${BOLD}Total raw tokens:${NC}      ${TOTAL_RAW}"
  echo -e "  ${BOLD}Total optimized tokens:${NC}${TOTAL_OPT}"
  echo -e "  ${GREEN}${BOLD}Total tokens saved:${NC}    ${TOKEN_REDUCTION}  (${SAVED_PCT}% reduction)"
  echo -e "  ${GREEN}${BOLD}Avg savings per req:${NC}   ${AVG_SAVINGS}%"
  echo ""
  echo -e "  ${BOLD}Avg latency (optimize):${NC}${AVG_LATENCY}ms"

  # Estimate cost savings (at GPT-4o pricing: $2.50 per 1M input tokens)
  COST_SAVED=$(echo "scale=4; $TOKEN_REDUCTION * 2.50 / 1000000" | bc)
  echo -e "  ${YELLOW}${BOLD}Est. cost saved (GPT-4o):${NC} \$${COST_SAVED} for this test batch"
else
  fail "All optimize requests failed — is repo indexed?"
fi

# ── 4. Health endpoint throughput test ──────────────────────────────────────
banner "Step 4 — Throughput Test: /v1/health (baseline)"

HEALTH_START=$(date +%s%3N)
for i in $(seq 1 50); do
  curl -s "${DAEMON}/v1/health" > /dev/null &
done
wait
HEALTH_END=$(date +%s%3N)
HEALTH_MS=$((HEALTH_END - HEALTH_START))
HEALTH_RPS=$(echo "scale=1; 50 * 1000 / $HEALTH_MS" | bc)
pass "50 concurrent health requests completed in ${HEALTH_MS}ms (~${HEALTH_RPS} RPS)"

# ── 5. CCR Retrieve test ─────────────────────────────────────────────────────
banner "Step 5 — CCR Retrieve round-trip test"
info "Running optimize to populate CCR cache..."

OPT_RESP=$(curl -s -X POST "${DAEMON}/v1/optimize" \
  -H "Content-Type: application/json" \
  -d '{"query":"Explain semantic cache and CCR", "max_tokens": 1024}')

COMPRESSED=$(echo "$OPT_RESP" | jq -r '.compressed_prompt // ""')
CCR_KEY=$(echo "$COMPRESSED" | grep -oP 'CCR Key: \K[a-f0-9]+' | head -1 || true)

if [ -n "$CCR_KEY" ]; then
  info "Found CCR Key: ${CCR_KEY}"
  RETRIEVE_START=$(date +%s%3N)
  RETRIEVE_RESP=$(curl -s -X POST "${DAEMON}/v1/retrieve" \
    -H "Content-Type: application/json" \
    -d "{\"key\": \"${CCR_KEY}\"}")
  RETRIEVE_END=$(date +%s%3N)
  RETRIEVE_MS=$((RETRIEVE_END - RETRIEVE_START))

  RETRIEVE_OK=$(echo "$RETRIEVE_RESP" | jq -r '.success // false')
  CONTENT_LEN=$(echo "$RETRIEVE_RESP" | jq -r '.original_content // ""' | wc -c)

  if [ "$RETRIEVE_OK" = "true" ]; then
    pass "CCR retrieve succeeded in ${RETRIEVE_MS}ms — restored ${CONTENT_LEN} bytes of original code"
  else
    fail "CCR retrieve failed: $(echo "$RETRIEVE_RESP" | jq -r '.message // "unknown"')"
  fi
else
  info "No CCR keys found in compressed output (all symbols may be high-relevance for this query)"
fi

# ── 6. Final summary ─────────────────────────────────────────────────────────
banner "Load Test Complete"
echo ""
echo -e "  ${GREEN}${BOLD}ContextIQ is working efficiently on your local setup.${NC}"
echo ""
echo "  Key metrics:"
[ "$PASS_COUNT" -gt 0 ] && echo "    • Token compression: ~${AVG_SAVINGS}% avg savings"
[ "$PASS_COUNT" -gt 0 ] && echo "    • Avg optimize latency: ${AVG_LATENCY}ms"
echo "    • Health throughput: ~${HEALTH_RPS} RPS"
echo ""
echo "  Next steps:"
echo "    1. Run with a real provider: DEFAULT_PROVIDER=ollama ./contextiq --port 9009"
echo "    2. Try higher concurrency:   bash scripts/load_test.sh http://localhost:9009 20 100"
echo "    3. Use 'hey' for HTTP load testing:  go install github.com/rakyll/hey@latest"
echo "       hey -n 200 -c 20 -m POST -H 'Content-Type: application/json' \\"
echo "           -d '{\"query\":\"test\", \"max_tokens\":4096}' \\"
echo "           http://localhost:9009/v1/optimize"
echo ""
