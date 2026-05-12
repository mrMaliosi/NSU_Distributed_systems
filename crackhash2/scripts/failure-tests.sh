#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-"$PROJECT_ROOT/compose.yaml"}"
MANAGER_URL="${MANAGER_URL:-http://localhost:8081}"
REQUEST_TIMEOUT_SEC="${REQUEST_TIMEOUT_SEC:-240}"
NO_WORKER_WAIT_SEC="${NO_WORKER_WAIT_SEC:-12}"
CURL_MAX_TIME_SEC="${CURL_MAX_TIME_SEC:-20}"

PASS_COUNT=0
FAIL_COUNT=0

log() {
  printf '[%s] %s\n' "$(date +'%H:%M:%S')" "$*"
}

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  log "PASS: $*"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  log "FAIL: $*"
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "Required command not found: $1"
  fi
}

json_get() {
  local key="$1"
  python3 -c 'import json,sys; print(json.loads(sys.stdin.read()).get(sys.argv[1], ""))' "$key"
}

post_request() {
  local hash="$1"
  local max_length="$2"
  local alphabet="$3"
  local payload
  payload=$(cat <<EOF
{"hash":"$hash","maxLength":$max_length,"algorithm":"MD5","alphabet":"$alphabet"}
EOF
)
  curl --max-time "$CURL_MAX_TIME_SEC" -fsS -X POST "$MANAGER_URL/api/hash/crack" \
    -H "Content-Type: application/json" \
    -d "$payload"
}

get_status_json() {
  local request_id="$1"
  curl --max-time "$CURL_MAX_TIME_SEC" -fsS "$MANAGER_URL/api/hash/status?requestId=$request_id"
}

get_status() {
  local request_id="$1"
  get_status_json "$request_id" | json_get status
}

wait_http() {
  local timeout="${1:-90}"
  local start
  start=$(date +%s)

  until curl --max-time "$CURL_MAX_TIME_SEC" -fsS "$MANAGER_URL/api/metrics" >/dev/null 2>&1; do
    if (( "$(date +%s)" - start > timeout )); then
      fail "Manager HTTP is unavailable for ${timeout}s"
    fi
    sleep 2
  done
}

wait_status() {
  local request_id="$1"
  local expected="$2"
  local timeout="$3"
  local start
  start=$(date +%s)

  while true; do
    local current
    current=$(get_status "$request_id")
    if [[ "$current" == "$expected" ]]; then
      return 0
    fi
    if (( "$(date +%s)" - start > timeout )); then
      log "Final status payload:"
      get_status_json "$request_id" || true
      fail "Timeout waiting status=$expected for request=$request_id, got=$current"
    fi
    sleep 2
  done
}

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

compose_up_base() {
  compose up -d --build --scale worker=2
  wait_http 120
}

compose_cleanup() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true
}

prepare_env() {
  require_cmd docker
  require_cmd curl
  require_cmd python3
  compose_cleanup
  compose_up_base
}

create_heavy_request() {
  local random_hash
  random_hash=$(python3 -c 'import uuid; print(uuid.uuid4().hex)')
  # Случайный hash гарантирует новый requestId и обычно не находится в диапазоне подбора.
  post_request "$random_hash" 6 "abcdefghijklmnopqrstuvwxyz0123456789"
}

create_worker_recovery_request() {
  local target_hash
  target_hash=$(python3 -c 'import hashlib; print(hashlib.md5("ffffff".encode()).hexdigest())')
  post_request "$target_hash" 6 "abcdef"
}

create_no_worker_request() {
  local random_hash
  random_hash=$(python3 -c 'import uuid; print(uuid.uuid4().hex)')
  post_request "$random_hash" 5 "abcdef"
}

test_manager_stop() {
  log "CASE 1: Stop manager service"
  local req_json req_id
  req_json=$(create_heavy_request)
  req_id=$(printf '%s' "$req_json" | json_get requestId)
  [[ -n "$req_id" ]] || fail "Manager did not return requestId"

  compose stop manager >/dev/null
  sleep 5
  compose start manager >/dev/null
  wait_http 90

  local status
  status=$(get_status "$req_id")
  [[ "$status" =~ ^(IN_PROGRESS|READY)$ ]] || fail "Unexpected status after manager restart: $status"
  pass "Manager restart preserves request data ($req_id, status=$status)"
}

test_dispatcher_stop() {
  log "CASE 2: Stop task dispatcher"
  local req_json req_id status_before status_after
  req_json=$(create_heavy_request)
  req_id=$(printf '%s' "$req_json" | json_get requestId)

  compose stop dispatcher >/dev/null
  sleep 8
  status_before=$(get_status "$req_id")
  [[ "$status_before" == "IN_PROGRESS" ]] || fail "Expected IN_PROGRESS while dispatcher is down, got: $status_before"

  compose start dispatcher >/dev/null
  sleep 4
  status_after=$(get_status "$req_id")
  [[ "$status_after" =~ ^(IN_PROGRESS|READY)$ ]] || fail "Unexpected status after dispatcher restart: $status_after"
  pass "Dispatcher restart does not lose request state ($req_id)"
}

discover_primary_service() {
  local primary_host
  primary_host=$(compose exec -T mongodb1 mongosh --quiet --eval "rs.status().members.find(m=>m.stateStr=='PRIMARY').name" | tr -d '"\r')
  case "$primary_host" in
    mongodb1:27017) echo "mongodb1" ;;
    mongodb2:27017) echo "mongodb2" ;;
    mongodb3:27017) echo "mongodb3" ;;
    *)
      fail "Unable to detect PRIMARY host from rs.status(): $primary_host"
      ;;
  esac
}

test_mongo_primary_stop() {
  log "CASE 3: Stop MongoDB PRIMARY node"
  local primary req_json req_id
  primary=$(discover_primary_service)
  log "Current PRIMARY: $primary"

  compose stop "$primary" >/dev/null
  sleep 12

  req_json=$(post_request "098f6bcd4621d373cade4e832627b4f6" 4 "abcdefghijklmnopqrstuvwxyz")
  req_id=$(printf '%s' "$req_json" | json_get requestId)
  [[ -n "$req_id" ]] || fail "No requestId after PRIMARY stop"

  compose start "$primary" >/dev/null
  pass "Request accepted during PRIMARY failover ($req_id)"
}

test_rabbitmq_stop() {
  log "CASE 4: Stop RabbitMQ"
  local req_json req_id status_during status_after
  req_json=$(create_heavy_request)
  req_id=$(printf '%s' "$req_json" | json_get requestId)

  compose stop rabbitmq >/dev/null
  sleep 10
  status_during=$(get_status "$req_id")
  [[ "$status_during" == "IN_PROGRESS" ]] || fail "Expected IN_PROGRESS while RabbitMQ down, got: $status_during"

  compose start rabbitmq >/dev/null
  sleep 10
  status_after=$(get_status "$req_id")
  [[ "$status_after" =~ ^(IN_PROGRESS|READY)$ ]] || fail "Unexpected status after RabbitMQ restart: $status_after"
  pass "RabbitMQ outage does not lose request ($req_id)"
}

worker_container_id() {
  compose ps -q worker | python3 -c 'import sys; lines=[x.strip() for x in sys.stdin if x.strip()]; print(lines[0] if lines else "")'
}

test_worker_stop_during_processing() {
  log "CASE 5: Stop worker during processing"
  local req_json req_id wid status_mid
  compose up -d --scale worker=1 >/dev/null
  sleep 2

  req_json=$(create_worker_recovery_request)
  req_id=$(printf '%s' "$req_json" | json_get requestId)
  sleep 1

  wid=$(worker_container_id)
  [[ -n "$wid" ]] || fail "No worker container found"
  docker stop "$wid" >/dev/null
  sleep 3

  status_mid=$(get_status "$req_id")
  [[ "$status_mid" == "IN_PROGRESS" ]] || fail "Expected IN_PROGRESS after worker stop, got: $status_mid"

  compose up -d --scale worker=1 >/dev/null

  wait_status "$req_id" "READY" "$REQUEST_TIMEOUT_SEC"
  pass "Request completed despite worker failure ($req_id)"
}

test_no_workers_at_creation() {
  log "CASE 6: No workers at task creation"
  compose up -d --scale worker=0 >/dev/null

  local req_json req_id status_before
  req_json=$(create_no_worker_request)
  req_id=$(printf '%s' "$req_json" | json_get requestId)
  sleep "$NO_WORKER_WAIT_SEC"

  status_before=$(get_status "$req_id")
  [[ "$status_before" == "IN_PROGRESS" ]] || fail "Expected IN_PROGRESS without workers, got: $status_before"

  compose up -d --scale worker=2 >/dev/null
  wait_status "$req_id" "READY" "$REQUEST_TIMEOUT_SEC"
  pass "Request waits without workers and completes after scale-up ($req_id)"
}

main() {
  prepare_env
  test_manager_stop
  test_dispatcher_stop
  test_mongo_primary_stop
  test_rabbitmq_stop
  test_worker_stop_during_processing
  test_no_workers_at_creation

  log "All done: PASS=$PASS_COUNT FAIL=$FAIL_COUNT"
  compose_cleanup
}

trap compose_cleanup EXIT
main "$@"
