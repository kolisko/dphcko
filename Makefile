.PHONY: build test verify dist clean

build:
	CGO_ENABLED=0 go build -trimpath -o build/dphcko ./cmd/dphcko

test:
	go test ./...

verify: test
	go vet ./...
	./scripts/validate-xsd.sh

dist:
	goreleaser release --snapshot --clean

clean:
	go clean
