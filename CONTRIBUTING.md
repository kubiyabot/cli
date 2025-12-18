# Contributing to Kubiya CLI

Thank you for your interest in contributing to the Kubiya CLI! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Describe the behavior you observed and what you expected**
- **Include your environment details** (OS, Go version, CLI version)
- **Include any relevant logs or error messages**

### Suggesting Enhancements

Enhancement suggestions are welcome! Please provide:

- **A clear and descriptive title**
- **A detailed description of the proposed enhancement**
- **Explain why this enhancement would be useful**
- **List any alternatives you've considered**

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Follow the coding style** used in the project
3. **Write clear, descriptive commit messages**
4. **Include tests** for any new functionality
5. **Update documentation** as needed
6. **Ensure all tests pass** before submitting

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional, for using Makefile commands)

### Building from Source

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/cli.git
cd cli

# Install dependencies
go mod download

# Build the CLI
go build -o kubiya .

# Run tests
go test ./...
```

### Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` to format your code
- Use `golint` and `go vet` to check for issues
- Write meaningful comments for exported functions and types

### Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests when relevant

### Testing

- Write unit tests for new functionality
- Ensure existing tests pass
- Test edge cases and error conditions
- Run the full test suite before submitting:

```bash
go test ./...
```

## Project Structure

```
cli/
├── internal/       # Internal packages
├── docs/           # Documentation
├── examples/       # Example configurations
├── test/           # Test files
└── main.go         # Entry point
```

## Getting Help

- Check the [documentation](docs/)
- Open an issue for questions
- Join our community channels

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
