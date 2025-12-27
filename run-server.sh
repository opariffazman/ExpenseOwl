#!/usr/bin/env bash

# ExpenseOwl Local Server Runner
# Usage: ./run-server.sh [port]
# Default port: 8081

set -e

PORT=${1:-8081}
LOG_FILE="$HOME/expenseowl.log"

echo "=== ExpenseOwl Server Runner ==="
echo ""

# Stop existing server
echo "Stopping existing server..."
pkill -f "go run cmd/expenseowl/main.go" 2>/dev/null || true

# Also kill any processes using the target port
ps aux | grep -E "${PORT}|go run cmd/expenseowl|expenseowl.*main" | grep -v grep | awk '{print $2}' | xargs -r kill 2>/dev/null || true

# Wait for processes to fully terminate
sleep 3

# Final check if port is available
PORT_CHECK=$(ps aux | grep -E ":${PORT}" | grep -v grep || true)
if [ -n "$PORT_CHECK" ]; then
    echo "Warning: Port $PORT may still be in use. Proceeding anyway..."
fi

# Start server
echo "Starting ExpenseOwl server on port $PORT..."
nohup go run cmd/expenseowl/main.go -port $PORT > $LOG_FILE 2>&1 &
SERVER_PID=$!

# Wait for server to start
sleep 3

# Check if server started successfully
if ps -p $SERVER_PID > /dev/null 2>&1; then
    echo ""
    echo "✓ Server started successfully!"
    echo "  URL:        http://localhost:$PORT"
    echo "  PID:        $SERVER_PID"
    echo "  Log file:   $LOG_FILE"
    echo ""
    echo "Recent logs:"
    echo "---"
    tail -5 $LOG_FILE
    echo "---"
    echo ""
    echo "To view logs: tail -f $LOG_FILE"
    echo "To stop:      pkill -f 'go run cmd/expenseowl/main.go'"
else
    echo ""
    echo "✗ Server failed to start. Check logs:"
    tail -10 $LOG_FILE
    exit 1
fi
