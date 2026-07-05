GOBIN := $(shell go env GOPATH)/bin

.PHONY: build tools fmt fmt-check vet lint vuln test check clean

build:
	go build -trimpath -o bin/spanner ./cmd/spanner

tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

fmt:
	gofmt -s -w .
	$(GOBIN)/goimports -local github.com/zandoh/spanner -w .

fmt-check:
	@out="$$(gofmt -s -l .)"; if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi
	@out="$$($(GOBIN)/goimports -local github.com/zandoh/spanner -l .)"; if [ -n "$$out" ]; then echo "goimports needed on:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

# staticcheck, gosec, errcheck, and depguard (strict import allowlist) all
# run under golangci-lint; see .golangci.yml.
lint:
	$(GOBIN)/golangci-lint run

vuln:
	$(GOBIN)/govulncheck ./...

test:
	go test -race -cover ./...

check: fmt-check vet lint vuln test

clean:
	rm -rf bin
