#!/bin/bash

# End-to-End Test for exec --local exit behavior
# This test verifies that the CLI exits cleanly after agent completion
# instead of hanging due to blocked signal handler goroutine

set -e

echo "🧪 E2E Test: exec --local Exit Behavior"
echo "========================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
TIMEOUT_SECONDS=180
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Check prerequisites
if [ -z "$KUBIYA_API_KEY" ]; then
    echo -e "${RED}❌ KUBIYA_API_KEY not set${NC}"
    echo "Please set KUBIYA_API_KEY environment variable"
    exit 1
fi

# Build the CLI
echo -e "${YELLOW}Step 1: Building CLI...${NC}"
cd "$PROJECT_ROOT"
go build -o /tmp/kubiya-test-exec-local .
echo -e "${GREEN}✓ CLI built successfully${NC}"
echo ""

# Test function
run_test() {
    local test_name="$1"
    local task="$2"
    local expected_behavior="$3"

    echo -e "${YELLOW}Test: $test_name${NC}"
    echo "Task: $task"
    echo "Expected: $expected_behavior"
    echo ""

    local start_time=$(date +%s)

    # Run with timeout
    set +e
    timeout "$TIMEOUT_SECONDS" /tmp/kubiya-test-exec-local exec --local --yes --compact "$task" > /tmp/exec_local_test_output.txt 2>&1
    local exit_code=$?
    set -e

    local end_time=$(date +%s)
    local elapsed=$((end_time - start_time))

    if [ $exit_code -eq 124 ]; then
        echo -e "${RED}❌ FAILED: Process HUNG and was killed after ${TIMEOUT_SECONDS}s${NC}"
        echo "This indicates the signal handler goroutine fix is NOT working!"
        echo ""
        echo "Last 50 lines of output:"
        tail -50 /tmp/exec_local_test_output.txt
        return 1
    elif [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}✓ PASSED: Process exited cleanly with code 0 in ${elapsed}s${NC}"
    else
        echo -e "${YELLOW}⚠ Process exited with code $exit_code in ${elapsed}s (not hanging = good)${NC}"
    fi

    # Show key output
    if grep -q "Worker shut down" /tmp/exec_local_test_output.txt 2>/dev/null; then
        echo -e "${GREEN}✓ Worker shutdown message found${NC}"
    fi

    echo ""
    return 0
}

# Run tests
echo -e "${YELLOW}Step 2: Running exit behavior tests...${NC}"
echo ""

FAILED=0

# Test 1: Simple echo task
if ! run_test "Simple Task Exit" \
    "echo hello world" \
    "Should exit with code 0 without hanging"; then
    FAILED=1
fi

# Test 2: Slightly more complex task
if ! run_test "File Listing Task Exit" \
    "list files in the current directory" \
    "Should exit cleanly after completion"; then
    FAILED=1
fi

# Summary
echo ""
echo "========================================"
echo -e "${YELLOW}Test Summary${NC}"
echo "========================================"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests PASSED${NC}"
    echo ""
    echo "The exec --local command exits cleanly without hanging."
    echo "The signal handler goroutine fix is working correctly."
    exit 0
else
    echo -e "${RED}❌ Some tests FAILED${NC}"
    echo ""
    echo "The CLI may still be hanging after agent completion."
    echo "Check the signal handler cleanup in exec.go"
    exit 1
fi
