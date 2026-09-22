# Optional convenience targets. Requires `make`, which is pre-installed on
# macOS and most Linux distributions. On Windows, install it via WSL,
# Git Bash, or MSYS2 — or just run the underlying `go` commands directly
# (see README.md for the plain-`go` equivalents on every OS).

.PHONY: build test vet fmt clean

BINARY := youtube-outliers

build:
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

clean:
	rm -f $(BINARY)
