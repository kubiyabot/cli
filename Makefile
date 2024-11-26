.PHONY: test test-setup test-teammates

test: test-setup test-teammates

test-setup:
	@echo "Setting up test environment..."
	@bash test/setup.sh

test-teammates:
	@echo "Running teammate tests..."
	@KUBIYA_TEST_MODE=true bats test/teammates.bats

test-clean:
	@echo "Cleaning up test environment..."
	@rm -rf test/test_helper/bats-* 