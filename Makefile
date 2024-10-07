# Variables
SWAGGER_FILE=swagger.yaml
CLIENT_DIR=kubiya
GENERATOR_CMD=openapi-generator
GENERATOR_LANG=go

.PHONY: all generate-client generate-commands build clean

# Default target
all: clean generate-client generate-commands build

# Generate the Go client from Swagger
generate-client:
	@echo "Generating Go client from $(SWAGGER_FILE)..."
	$(GENERATOR_CMD) generate \
		-i $(SWAGGER_FILE) \
		-g $(GENERATOR_LANG) \
		-o $(CLIENT_DIR) \
		--additional-properties=packageName=kubiya

# Generate Cobra command files
generate-commands:
	@echo "Generating Cobra commands..."
	go run cmd_generator/cmd_generator.go

# Build the CLI executable
build:
	@echo "Building kubiya-cli..."
	go build -o kubiya-cli main.go

# Clean generated files
clean:
	@echo "Cleaning up..."
	rm -rf $(CLIENT_DIR)
	rm -rf cmd/*.go
	rm -f kubiya-cli