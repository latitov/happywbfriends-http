
.PHONY: unittest
unittest:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-docker
lint-docker:
	docker run --rm -v "${CURDIR}":/app:ro -w /app golangci/golangci-lint:v1.50.1 golangci-lint run -v ./...
