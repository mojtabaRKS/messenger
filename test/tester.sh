#!/bin/bash
set -e

# -------------------------------
# TRACK BACKGROUND PROCESSES
# -------------------------------
PIDS=()

cleanup() {
    echo ""
    echo "== Cleaning up background services =="
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" >/dev/null 2>&1; then
            echo "Killing PID: $pid"
            kill "$pid" >/dev/null 2>&1 || true
        fi
    done
    echo "== Cleanup complete =="
}

# Trap CTRL+C and EXIT (normal end or error)
trap cleanup INT EXIT

#echo "== Step 0: Docker compose down (remove volumes) =="
#docker compose down --volumes

#echo "== Step 1: Docker compose up (detached) =="
#docker compose up -d

echo "== Step 2: Waiting for containers to become healthy =="

# --- Postgres ---
echo -n "Waiting for Postgres..."
until docker exec postgres pg_isready -U messenger >/dev/null 2>&1; do
  echo -n "."
  sleep 1
done
echo " OK"

# --- Redis ---
echo -n "Waiting for Redis..."
until docker exec redis redis-cli ping 2>/dev/null | grep -q "PONG"; do
  echo -n "."
  sleep 1
done
echo " OK"

# --- Zookeeper ---
echo -n "Waiting for Zookeeper..."
until nc -z localhost 2181 >/dev/null 2>&1; do
  echo -n "."
  sleep 1
done
echo " OK"

# --- Kafka ---
echo -n "Waiting for Kafka..."
until echo > /dev/tcp/localhost/9092 2>/dev/null; do
  echo -n "."
  sleep 1
done
echo " OK"

# --- ClickHouse ---
echo -n "Waiting for ClickHouse..."
until curl -s http://localhost:8123 >/dev/null 2>&1; do
  echo -n "."
  sleep 1
done
echo " OK"

echo "== All services are UP and READY =="
sleep 1

echo "== remove log files =="
[ -f ./logs/logs-consume.log ] && rm ./logs/logs-consume.log
[ -f ./logs/logs-server.log ] && rm ./logs/logs-server.log
[ -f ./logs/logs-consume-status.log ] && rm ./logs/logs-consume-status.log
echo "== logs remove was successful =="

echo "== Step 3: Running migrations =="
go run ./cmd/main.go migrate up
echo "== migrations was successful =="

echo "== Step 4: Simulate charge =="
go run ./test/simulator.go charge

echo "== Step 5: Start consumer (background) =="
go run ./cmd/main.go consume > ./logs/logs-consume.log 2>&1 &
PIDS+=($!)

echo "== Step 6: Start server (background) =="
go run ./cmd/main.go server > ./logs/logs-server.log 2>&1 &
PIDS+=($!)

echo "== Step 7: Start status consumer (background) =="
go run ./cmd/main.go consume-status > ./logs/logs-consume-status.log 2>&1 &
PIDS+=($!)

echo "== Step 8: Simulate run =="
go run ./test/simulator.go run

echo "== Everything is working. Background services running. =="

# This keeps trap active during runtime
wait
