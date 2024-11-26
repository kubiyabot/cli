#!/usr/bin/env bash

# Load bats-support and bats-assert libraries
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'

# Test environment setup
setup_test_env() {
    # Set test environment variables
    export KUBIYA_API_KEY="test-api-key"
    export KUBIYA_BASE_URL="https://api.kubiya.ai/api/v1"
    export KUBIYA_TEST_MODE="true"
    
    # Create temporary test directory
    export TEST_DIR="$(mktemp -d)"
    
    # Copy fixtures to test directory
    cp -r "${BATS_TEST_DIRNAME}/fixtures/"* "${TEST_DIR}/"
}

# Test environment cleanup
teardown_test_env() {
    rm -rf "${TEST_DIR}"
    unset KUBIYA_API_KEY
    unset KUBIYA_BASE_URL
    unset KUBIYA_TEST_MODE
    unset TEST_DIR
}

# Load test prompts
load_test_prompts() {
    local prompts_file="${BATS_TEST_DIRNAME}/fixtures/prompts.txt"
    while IFS='=' read -r key value; do
        if [[ $key != \#* && -n $key ]]; then
            declare -g "PROMPT_${key}=${value}"
        fi
    done < "$prompts_file"
}

# Create test files with specific content
create_test_file() {
    local path="$1"
    local content="$2"
    mkdir -p "$(dirname "${TEST_DIR}/${path}")"
    echo "$content" > "${TEST_DIR}/${path}"
}

# Create sample test files
create_sample_files() {
    # Create a Go file
    create_test_file "test.go" '
package main

func main() {
    println("Hello, World!")
}
'

    # Create a Dockerfile
    create_test_file "Dockerfile" '
FROM golang:1.18
COPY . /app
WORKDIR /app
RUN go build -o main .
CMD ["./main"]
'

    # Create a YAML config
    create_test_file "config.yaml" '
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  ports:
    - port: 80
'
} 