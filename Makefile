build: cmd/prguy/*.go
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/$(shell basename $(PWD))_amd64 ./cmd/prguy/
	GOOS=darwin GOARCH=arm64 go build -o bin/$(shell basename $(PWD))_arm64 ./cmd/prguy/

fmt:
	go fmt ./...
