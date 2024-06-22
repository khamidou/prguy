build: cmd/prguy/*.go
	go build -o bin/$(shell basename $(PWD)) ./cmd/prguy/

fmt:
	go fmt ./...
