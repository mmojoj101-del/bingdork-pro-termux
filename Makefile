.PHONY: build clean test lint bench all run docker help

APP_NAME := bingdork
BUILD_DIR := build
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_VERSION := $(shell go version | awk '{print $$3}')
LDFLAGS := -ldflags "-X github.com/bingdork/bingdork/cli.Version=${VERSION} -X github.com/bingdork/bingdork/cli.Commit=${COMMIT} -X github.com/bingdork/bingdork/cli.Date=${DATE} -X github.com/bingdork/bingdork/cli.GoVersion=${GO_VERSION}"

all: clean deps lint test build

build:
	@echo "Building ${APP_NAME} ${VERSION}..."
	@mkdir -p ${BUILD_DIR}
	CGO_ENABLED=0 go build ${LDFLAGS} -o ${BUILD_DIR}/${APP_NAME} ./cmd/${APP_NAME}/
	@echo "Build complete: ${BUILD_DIR}/${APP_NAME}"

build-all:
	@echo "Cross-compiling..."
	@mkdir -p ${BUILD_DIR}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ${LDFLAGS} -o ${BUILD_DIR}/${APP_NAME}-linux-amd64 ./cmd/${APP_NAME}/
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ${LDFLAGS} -o ${BUILD_DIR}/${APP_NAME}-linux-arm64 ./cmd/${APP_NAME}/
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build ${LDFLAGS} -o ${BUILD_DIR}/${APP_NAME}-darwin-amd64 ./cmd/${APP_NAME}/
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ${LDFLAGS} -o ${BUILD_DIR}/${APP_NAME}-darwin-arm64 ./cmd/${APP_NAME}/
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ${LDFLAGS} -o ${BUILD_DIR}/${APP_NAME}-windows-amd64.exe ./cmd/${APP_NAME}/
	@echo "Cross-compilation complete"

run:
	@go run ./cmd/${APP_NAME}/ $(ARGS)

test:
	@echo "Running tests..."
	go test -v -race -count=1 ./...
	@echo "Tests complete"

test-short:
	go test -v -short ./...

test-race:
	go test -v -race -count=1 ./...

bench:
	go test -bench=. -benchmem ./...

lint:
	@echo "Running linter..."
	golangci-lint run ./...
	@echo "Lint passed"

vet:
	go vet ./...

clean:
	@echo "Cleaning..."
	rm -rf ${BUILD_DIR}
	rm -rf tmp/
	go clean -cache
	@echo "Clean complete"

deps:
	@echo "Tidying dependencies..."
	go mod tidy
	go mod verify
	@echo "Dependencies verified"

update-deps:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy
	@echo "Dependencies updated"

docker:
	@echo "Building Docker image..."
	docker build -t ${APP_NAME}:${VERSION} -t ${APP_NAME}:latest .
	@echo "Docker image built: ${APP_NAME}:${VERSION}"

docker-run:
	docker run --rm -it ${APP_NAME}:latest $(ARGS)

coverage:
	@echo "Running tests with coverage..."
	mkdir -p ${BUILD_DIR}
	go test -v -race -coverprofile=${BUILD_DIR}/coverage.out -covermode=atomic ./...
	go tool cover -html=${BUILD_DIR}/coverage.out -o ${BUILD_DIR}/coverage.html
	go tool cover -func=${BUILD_DIR}/coverage.out
	@echo "Coverage report: ${BUILD_DIR}/coverage.html"

profile:
	@echo "Running CPU profile..."
	go test -bench=. -cpuprofile=${BUILD_DIR}/cpu.prof ./...
	go tool pprof -pdf ${BUILD_DIR}/cpu.prof > ${BUILD_DIR}/cpu.pdf
	@echo "CPU profile: ${BUILD_DIR}/cpu.pdf"

fuzz:
	@echo "Running fuzz tests..."
	go test -fuzz=. -fuzztime=30s ./...

install:
	@echo "Installing ${APP_NAME}..."
	go install ${LDFLAGS} ./cmd/${APP_NAME}/
	@echo "Installed: ${GOPATH}/bin/${APP_NAME}"

uninstall:
	@rm -f ${GOPATH}/bin/${APP_NAME}
	@echo "Uninstalled: ${APP_NAME}"

release: clean lint test build-all
	@echo "Building release packages..."
	@cd ${BUILD_DIR} && for f in ${APP_NAME}-*; do \
		tar czf "$${f}.tar.gz" "$$f"; \
		sha256sum "$${f}.tar.gz" > "$${f}.tar.gz.sha256"; \
	done
	@echo "Release packages ready in ${BUILD_DIR}/"

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all          - Clean, deps, lint, test, build (default)"
	@echo "  build        - Build binary"
	@echo "  build-all    - Cross-compile for all platforms"
	@echo "  test         - Run tests with race detection"
	@echo "  test-short   - Run short tests"
	@echo "  bench        - Run benchmarks"
	@echo "  lint         - Run golangci-lint"
	@echo "  clean        - Remove build artifacts"
	@echo "  deps         - Tidy and verify dependencies"
	@echo "  docker       - Build Docker image"
	@echo "  docker-run   - Run in Docker"
	@echo "  coverage     - Generate coverage report"
	@echo "  install      - Install binary"
	@echo "  release      - Build release packages"
	@echo "  run          - Run with ARGS='...'"
	@echo "  profile      - CPU profiling"
	@echo "  fuzz         - Run fuzz tests"
	@echo ""
	@echo "Variables:"
	@echo "  ARGS         - Arguments for run target"
	@echo "  VERSION      - Version string (default: git describe)"
