BINARY := micgain-manager
DIST := dist

.PHONY: build clean run

build:
	mkdir -p $(DIST)
	GOOS=darwin GOARCH=arm64 go build -o $(DIST)/$(BINARY) ./cmd/micgain-manager

run: build
	$(DIST)/$(BINARY) serve

clean:
	rm -rf $(DIST)
