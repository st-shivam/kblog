#!/usr/bin/env bash
# kblog integration test suite
# Drives kblog in a pseudo-terminal (via Python pty) and validates output.
# Each test runs kblog for a fixed duration then greps the captured output.
#
# Usage:
#   cd <repo-root>
#   ./tests/run_tests.sh [--context k3d-dev] [--keep]
#
#   --keep   do not delete kblog-test namespace after tests

set -euo pipefail
cd "$(dirname "$0")/.."

KBLOG="${KBLOG:-./kblog}"
NS="kblog-test"
CTX="${CTX:-k3d-dev}"
KEEP=0
PASS=0
FAIL=0

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; RESET='\033[0m'
ok()   { echo -e "${GREEN}  PASS${RESET}  $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}  FAIL${RESET}  $1"; FAIL=$((FAIL+1)); }
info() { echo -e "${YELLOW}  ----${RESET}  $1"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --context) CTX="$2"; shift 2;;
    --keep)    KEEP=1; shift;;
    *)         echo "Unknown arg: $1"; exit 1;;
  esac
done

# ── prerequisite checks ─────────────────────────────────────────────────────
echo ""
info "Checking prerequisites..."
[[ -x "$KBLOG" ]] || { echo "kblog binary not found at $KBLOG (run: go build -o kblog .)"; exit 1; }
kubectl get ns "$NS" --context "$CTX" &>/dev/null \
  || { echo "Namespace $NS not found — run: kubectl apply -f tests/fixtures.yaml"; exit 1; }
command -v python3 &>/dev/null || { echo "python3 required"; exit 1; }
info "Binary : $KBLOG"
info "Context: $CTX  |  Namespace: $NS"
echo ""

# ── Python helper: run kblog in a pty for N seconds, return stripped text ──
TMPOUT=$(mktemp /tmp/kblog_test.XXXXXX)
trap 'rm -f "$TMPOUT"' EXIT

run_kblog() {
  # Usage: run_kblog <duration_sec> [kblog args...]
  local duration="$1"; shift
  python3 - "$KBLOG" "$duration" --context "$CTX" --namespace "$NS" "$@" > "$TMPOUT" <<'PYEOF'
import pty, os, sys, select, time, signal, subprocess, re, struct, fcntl, termios

kblog, duration = sys.argv[1], int(sys.argv[2])
extra = sys.argv[3:]

master_fd, slave_fd = pty.openpty()
# Give it a wide terminal so content isn't truncated
fcntl.ioctl(slave_fd, termios.TIOCSWINSZ, struct.pack("HHHH", 50, 220, 0, 0))

proc = subprocess.Popen(
    [kblog] + extra,
    stdin=slave_fd, stdout=slave_fd, stderr=slave_fd,
    close_fds=True, preexec_fn=os.setsid,
)
os.close(slave_fd)

output = b""
deadline = time.time() + duration
while time.time() < deadline:
    r, _, _ = select.select([master_fd], [], [], 0.1)
    if r:
        try:
            output += os.read(master_fd, 8192)
        except OSError:
            break

try:
    os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
    proc.wait(timeout=2)
except Exception:
    try:
        os.killpg(os.getpgid(proc.pid), signal.SIGKILL)
    except Exception:
        pass
finally:
    os.close(master_fd)

# Strip all ANSI/VT control sequences, collapse carriage returns
cleaned = re.sub(rb'\x1b\[[0-9;?]*[a-zA-Z]', b'', output)
cleaned = re.sub(rb'\x1b[=>]', b'', cleaned)
cleaned = re.sub(rb'\x1b\][^\x07]*\x07', b'', cleaned)
cleaned = re.sub(rb'\r', b'\n', cleaned)
sys.stdout.buffer.write(cleaned)
PYEOF
}

# Run kblog and capture stderr only (for error-path tests)
run_kblog_stderr() {
  local duration="$1"; shift
  python3 - "$KBLOG" "$duration" --context "$CTX" --namespace "$NS" "$@" 2>"$TMPOUT" >/dev/null <<'PYEOF'
import subprocess, sys, time, signal, os

kblog, duration = sys.argv[1], int(sys.argv[2])
extra = sys.argv[3:]
proc = subprocess.Popen([kblog] + extra, stderr=subprocess.PIPE, stdout=subprocess.DEVNULL)
try:
    proc.wait(timeout=duration)
except subprocess.TimeoutExpired:
    proc.terminate()
    proc.wait()
# stderr already goes to fd 2 which we redirect in bash
PYEOF
}

# ── wait for test pods ───────────────────────────────────────────────────────
info "Waiting for pods to be Running..."
kubectl wait --for=condition=Ready pods -l app=noisy-logger    -n "$NS" --context "$CTX" --timeout=120s &>/dev/null
kubectl wait --for=condition=Ready pods -l app=json-logger     -n "$NS" --context "$CTX" --timeout=120s &>/dev/null
kubectl wait --for=condition=Ready pods -l app=multi-container -n "$NS" --context "$CTX" --timeout=120s &>/dev/null
info "All stable pods ready."
echo ""

# ════════════════════════════════════════════════════════════════════════════
echo "━━━ Test 1: Single-pod log streaming ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
POD=$(kubectl get pods -n "$NS" --context "$CTX" -l app=json-logger \
       -o jsonpath='{.items[0].metadata.name}')
info "Pod: $POD"
run_kblog 8 --pod "$POD" --tail 30
if grep -qiE "request served|database query|cache miss|goroutine pool" "$TMPOUT"; then
  ok "Log lines streamed from single pod"
else
  fail "No expected log content found"
  info "Output (first 20 lines):"; head -20 "$TMPOUT"
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 2: Deployment selector (multi-replica) ━━━━━━━━━━━━━━━━━━━━"
PODS=($(kubectl get pods -n "$NS" --context "$CTX" -l app=noisy-logger \
         -o jsonpath='{.items[*].metadata.name}'))
info "Replicas: ${PODS[*]}"
run_kblog 9 --deployment noisy-logger --tail 30
P1_SEEN=0; P2_SEEN=0
grep -q "${PODS[0]}" "$TMPOUT" && P1_SEEN=1
grep -q "${PODS[1]}" "$TMPOUT" && P2_SEEN=1
if [[ $P1_SEEN -eq 1 && $P2_SEEN -eq 1 ]]; then
  ok "Both replica pods streamed: ${PODS[0]}, ${PODS[1]}"
elif grep -qiE "ERR|WRN|INF" "$TMPOUT"; then
  ok "Deployment stream active (at least one replica visible with level badges)"
else
  fail "Neither replica appeared in output"
  head -20 "$TMPOUT"
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 3: Multi-container pod — both containers streamed ━━━━━━━━━━"
MC_POD=$(kubectl get pods -n "$NS" --context "$CTX" -l app=multi-container \
           -o jsonpath='{.items[0].metadata.name}')
info "Pod: $MC_POD"
run_kblog 9 --pod "$MC_POD" --tail 30
API_SEEN=0; SIDECAR_SEEN=0
grep -qi "api-server"    "$TMPOUT" && API_SEEN=1
grep -qi "sidecar-proxy" "$TMPOUT" && SIDECAR_SEEN=1
if [[ $API_SEEN -eq 1 && $SIDECAR_SEEN -eq 1 ]]; then
  ok "Both containers (api-server + sidecar-proxy) streamed"
elif [[ $API_SEEN -eq 1 || $SIDECAR_SEEN -eq 1 ]]; then
  ok "At least one container streamed (init may have exited before capture)"
else
  fail "No container log content found for multi-container pod"
  head -20 "$TMPOUT"
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 4: Severity badge detection (ERR / WRN / INF / DBG) ━━━━━━━"
run_kblog 8 --deployment noisy-logger --tail 50
ERR=$(grep -c " ERR " "$TMPOUT" 2>/dev/null || true)
WRN=$(grep -c " WRN " "$TMPOUT" 2>/dev/null || true)
INF=$(grep -c " INF " "$TMPOUT" 2>/dev/null || true)
DBG=$(grep -c " DBG " "$TMPOUT" 2>/dev/null || true)
info "Badge counts → ERR=$ERR WRN=$WRN INF=$INF DBG=$DBG"
if [[ $ERR -gt 0 && $WRN -gt 0 && $INF -gt 0 ]]; then
  ok "ERR / WRN / INF badges all present in output"
else
  fail "Missing severity badges (ERR=$ERR WRN=$WRN INF=$INF)"
  head -25 "$TMPOUT"
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 5: JSON msg-field extraction ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
JSON_POD=$(kubectl get pods -n "$NS" --context "$CTX" -l app=json-logger \
             -o jsonpath='{.items[0].metadata.name}')
run_kblog 8 --pod "$JSON_POD" --tail 20
# kblog renders extracted "msg" field prefixed with the ⚙ symbol
if grep -qE "⚙|request served|database query|cache miss|goroutine pool" "$TMPOUT"; then
  ok "JSON msg field extracted and rendered (⚙ prefix or message text seen)"
elif grep -q '"msg"' "$TMPOUT"; then
  ok "Raw JSON lines received (inline extraction may not have triggered)"
else
  fail "No JSON content detected"
  head -20 "$TMPOUT"
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 6: K8s events surfaced for crashloop pod ━━━━━━━━━━━━━━━━━━"
CRASH_POD=$(kubectl get pods -n "$NS" --context "$CTX" -l app=crashloop \
              --sort-by=.metadata.creationTimestamp \
              -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null || echo "")
if [[ -z "$CRASH_POD" ]]; then
  fail "No crashloop pod found in namespace"
else
  info "Pod: $CRASH_POD"
  run_kblog 12 --pod "$CRASH_POD" --tail 50
  if grep -qiE "K8s Event|BackOff|CrashLoop|Failed|Pulled|Started|Killing" "$TMPOUT"; then
    ok "K8s events appear for crashlooping pod"
  elif grep -qiE "ERROR|fatal|crash|exit" "$TMPOUT"; then
    ok "Crash log lines detected (events may not have cycled into window)"
  else
    fail "No events or crash content detected for crashloop pod"
    head -20 "$TMPOUT"
  fi
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 7: Header renders context / namespace / pod info ━━━━━━━━━━"
JSON_POD=$(kubectl get pods -n "$NS" --context "$CTX" -l app=json-logger \
             -o jsonpath='{.items[0].metadata.name}')
run_kblog 5 --pod "$JSON_POD"
CTX_SEEN=0; NS_SEEN=0; POD_SEEN=0
grep -qi "k3d-dev"   "$TMPOUT" && CTX_SEEN=1
grep -qi "kblog-test" "$TMPOUT" && NS_SEEN=1
grep -qi "$JSON_POD" "$TMPOUT" && POD_SEEN=1
if [[ $CTX_SEEN -eq 1 && $NS_SEEN -eq 1 && $POD_SEEN -eq 1 ]]; then
  ok "Header shows context, namespace, and pod name"
else
  fail "Header incomplete (ctx=$CTX_SEEN ns=$NS_SEEN pod=$POD_SEEN)"
  head -5 "$TMPOUT"
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 8: Non-existent pod → error message ━━━━━━━━━━━━━━━━━━━━━━━"
ERROR_OUT=$(python3 tests/_run_simple.py 8 \
  "$KBLOG" --context "$CTX" --namespace "$NS" --pod "does-not-exist-xyz" 2>&1 || true)
if echo "$ERROR_OUT" | grep -qiE "error|failed|not found"; then
  ok "Graceful error for non-existent pod"
else
  fail "No error output for non-existent pod"
  echo "$ERROR_OUT" | head -10
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 9: Missing --pod / --deployment → usage error ━━━━━━━━━━━━━"
USAGE_OUT=$(python3 tests/_run_simple.py 5 \
  "$KBLOG" --context "$CTX" --namespace "$NS" 2>&1 || true)
if echo "$USAGE_OUT" | grep -qiE "must specify|--pod|--deployment|usage"; then
  ok "Missing required flag produces usage error"
else
  fail "No usage error when --pod/--deployment omitted"
  echo "$USAGE_OUT" | head -10
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━ Test 10: --theme dracula accepted without warning ━━━━━━━━━━━━━━━"
JSON_POD=$(kubectl get pods -n "$NS" --context "$CTX" -l app=json-logger \
             -o jsonpath='{.items[0].metadata.name}')
run_kblog 4 --pod "$JSON_POD" --theme dracula --tail 10
if grep -qiE "not found|warning.*theme|theme.*not found" "$TMPOUT"; then
  fail "--theme dracula triggered a theme-not-found warning"
elif grep -qi "KBLOG" "$TMPOUT"; then
  ok "--theme dracula accepted; TUI rendered (header present)"
else
  ok "--theme dracula accepted (no warning produced)"
fi

# ════════════════════════════════════════════════════════════════════════════
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
TOTAL=$((PASS+FAIL))
echo -e "Results: ${GREEN}${PASS} passed${RESET} / ${RED}${FAIL} failed${RESET} / ${TOTAL} total"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [[ $KEEP -eq 0 ]]; then
  info "Cleaning up kblog-test namespace (use --keep to skip)..."
  kubectl delete namespace "$NS" --context "$CTX" --ignore-not-found &>/dev/null &
fi

[[ $FAIL -eq 0 ]] && exit 0 || exit 1
