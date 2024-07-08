build: cmd/prguy/*.go
	mkdir -p bin
	go build -o bin/prguy_arm64 ./cmd/prguy/

release: cmd/prguy/*.go
	mkdir -p bin
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/prguy_amd64 ./cmd/prguy/
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o bin/prguy_arm64 ./cmd/prguy/
	lipo -create -output bin/prguy bin/prguy_amd64 bin/prguy_arm64

fmt:
	go fmt ./...
