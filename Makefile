.PHONY: default test lint docker buildah release-artifacts clean

BINARY := prometheus-gmail-exporter
DIST := dist
VERSION ?= dev

default: $(BINARY)

$(BINARY):
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/prometheus-gmail-exporter/

release-artifacts: $(BINARY)
	mkdir -p $(DIST)
	cp $(BINARY) $(DIST)/
	echo "$(VERSION):$$(git rev-parse HEAD 2>/dev/null || echo unknown)" > $(DIST)/VERSION

test:
	go test ./...

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed; skipped"

docker:
	docker build . -t docker.io/jamesread/prometheus-gmail-exporter

buildah:
	buildah bud -t docker.io/jamesread/prometheus-gmail-exporter

devcerts:
	openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

clean:
	rm -f $(BINARY)
