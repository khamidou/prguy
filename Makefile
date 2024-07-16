build: cmd/prguy/*.go
	mkdir -p bin
	go build -o bin/prguy_arm64 ./cmd/prguy/

release: cmd/prguy/*.go
	mkdir -p bin
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/prguy_amd64 ./cmd/prguy/
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o bin/prguy_arm64 ./cmd/prguy/
	lipo -create -output bin/prguy bin/prguy_amd64 bin/prguy_arm64
	cp bin/prguy PRGuy.app/Contents/MacOS/
	codesign -f -o runtime --timestamp -s "Developer ID Application: Karim Hamidou (6NQ4A56YNV)" PRGuy.app
	zip -r prguy.zip PRGuy.app
	xcrun notarytool submit prguy.zip --keychain-profile "Karim"
	@echo "You can call 'xcrun notarytool info YOUR_ID --keychain-profile "Karim"' to check the status of the notarization"

fmt:
	go fmt ./...
