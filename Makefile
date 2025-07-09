.PHONY: build
build:
	go mod tidy
	#go mod vendor
	go build -o kubiya -ldflags="-s -w" main.go

.PHONY: install
install: build
	mv kubiya /usr/local/bin/kubiya

# .PHONY: test
# test:
# 	go test ./... 
