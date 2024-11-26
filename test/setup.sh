#!/bin/bash

# Install test dependencies
install_dependencies() {
    # Create test directories
    mkdir -p test/fixtures
    mkdir -p test/test_helper

    # Install bats-core if not already installed
    if ! command -v bats &> /dev/null; then
        git clone https://github.com/bats-core/bats-core.git
        cd bats-core
        ./install.sh /usr/local
        cd ..
        rm -rf bats-core
    fi

    # Install bats-support
    if [ ! -d "test/test_helper/bats-support" ]; then
        git clone https://github.com/bats-core/bats-support.git test/test_helper/bats-support
    fi

    # Install bats-assert
    if [ ! -d "test/test_helper/bats-assert" ]; then
        git clone https://github.com/bats-core/bats-assert.git test/test_helper/bats-assert
    fi
}

install_dependencies 