GOBIN := $(shell go env GOPATH)/bin

.PHONY: build tools fmt fmt-check vet lint sec vuln test check clean

build:
	go build -trimpath -o bin/spanner ./cmd/spanner

tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

fmt:
	gofmt -s -w .
	$(GOBIN)/goimports -local github.com/zandoh/spanner -w .

fmt-check:
	@out="$$(gofmt -s -l .)"; if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi
	@out="$$($(GOBIN)/goimports -local github.com/zandoh/spanner -l .)"; if [ -n "$$out" ]; then echo "goimports needed on:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

lint:
	$(GOBIN)/staticcheck ./...

vuln:
	$(GOBIN)/govulncheck ./...

sec:
	$(GOBIN)/gosec -quiet ./...

test:
	go test -race -cover ./...

check: fmt-check vet lint sec vuln test

clean:
	rm -rf bin
