APP := dbibackend

.PHONY: build test release clean

build:
	go build -ldflags="-s -w" -o $(APP) .

test:
	go test ./...

# dist/DBI Backend.app (universal) + dist/DBI Backend.exe, libusb linked statically
release:
	./scripts/release.sh

clean:
	rm -rf $(APP) .build dist
