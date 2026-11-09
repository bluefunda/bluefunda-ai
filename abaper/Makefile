BINARY_NAME=abaper
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/bluefunda/abaper/internal/commands.version=$(VERSION)"
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

MANDIR ?= /usr/local/share/man/man1

.PHONY: build build-all clean test lint vet fmt run install-man uninstall-man

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/abaper

run: build
	./bin/$(BINARY_NAME)

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

clean:
	rm -rf bin/ dist/

build-all:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) \
			-o bin/$(BINARY_NAME)-$$os-$$arch$$ext ./cmd/abaper; \
	done

install-man:
	install -d $(MANDIR)
	install -m 644 man/abaper.1 $(MANDIR)/abaper.1

uninstall-man:
	rm -f $(MANDIR)/abaper.1

docker-build:
	docker build -t $(BINARY_NAME) .

docker-run:
	docker run --rm $(BINARY_NAME) version
