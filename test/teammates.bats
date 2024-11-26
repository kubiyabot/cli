#!/usr/bin/env bats

load 'test_helper.bash'

setup() {
    setup_test_env
    create_sample_files
    load_test_prompts
}

teardown() {
    teardown_test_env
}

# DevOps teammate tests
@test "devops: kubernetes explanation" {
    run kubiya chat -n "DevOps Bot" -m "${PROMPT_explain_kubernetes}"
    assert_success
    assert_output --partial "Kubernetes"
    assert_output --partial "container"
    assert_output --partial "orchestration"
}

@test "devops: deployment review" {
    create_test_file "deployment.yaml" "$(cat ${BATS_TEST_DIRNAME}/fixtures/deployment.yaml)"
    run kubiya chat -n "DevOps Bot" \
        --context "${TEST_DIR}/deployment.yaml" \
        -m "${PROMPT_review_deployment}"
    assert_success
    assert_output --partial "review"
    assert_output --partial "deployment"
}

# Security teammate tests
@test "security: code audit" {
    run kubiya chat -n "security" \
        --context "${TEST_DIR}/test.go" \
        -m "${PROMPT_check_security}"
    assert_success
    assert_output --partial "security"
    assert_output --partial "analysis"
}

@test "security: dockerfile scan" {
    run kubiya chat -n "security" \
        --context "${TEST_DIR}/Dockerfile" \
        -m "${PROMPT_scan_dockerfile}"
    assert_success
    assert_output --partial "security"
    assert_output --partial "Docker"
}

# Documentation teammate tests
@test "docs: generate api docs" {
    run kubiya chat -n "docs" \
        --context "${TEST_DIR}/test.go" \
        -m "${PROMPT_generate_docs}"
    assert_success
    assert_output --partial "documentation"
    assert_output --partial "API"
}

@test "docs: readme generation" {
    run kubiya chat -n "docs" \
        --context "${TEST_DIR}/test.go" \
        -m "${PROMPT_write_readme}"
    assert_success
    assert_output --partial "README"
    assert_output --partial "Installation"
}

# Code review teammate tests
@test "code-review: performance optimization" {
    run kubiya chat -n "code-review" \
        --context "${TEST_DIR}/test.go" \
        -m "${PROMPT_optimize_performance}"
    assert_success
    assert_output --partial "performance"
    assert_output --partial "optimization"
}

# Error handling tests
@test "handles invalid teammate name" {
    run kubiya chat -n "nonexistent-teammate" -m "Hello"
    assert_failure
    assert_output --partial "teammate not found"
}

@test "handles missing API key" {
    KUBIYA_API_KEY="" run kubiya chat -n "DevOps Bot" -m "Hello"
    assert_failure
    assert_output --partial "API key"
}

@test "handles invalid file context" {
    run kubiya chat -n "DevOps Bot" \
        --context "nonexistent-file.txt" \
        -m "Review this"
    assert_failure
    assert_output --partial "file not found"
}

# Session management tests
@test "maintains conversation context" {
    # First message
    run kubiya chat -n "DevOps Bot" -m "Let's discuss CI/CD"
    assert_success
    SESSION_ID=$(echo "$output" | grep -o 'session-[0-9a-f]\+')
    
    # Follow-up message
    run kubiya chat -n "DevOps Bot" -m "Continue from before" -s "${SESSION_ID}"
    assert_success
    assert_output --partial "CI/CD"
}

# Complex interaction tests
@test "handles multiple file contexts" {
    run kubiya chat -n "DevOps Bot" \
        --context "${TEST_DIR}/Dockerfile" \
        --context "${TEST_DIR}/config.yaml" \
        -m "Review these files"
    assert_success
    assert_output --partial "Dockerfile"
    assert_output --partial "configuration"
}

@test "processes stdin input" {
    echo "Error: Connection refused" | \
    run kubiya chat -n "debug" --stdin
    assert_success
    assert_output --partial "error"
    assert_output --partial "connection"
} 