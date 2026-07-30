# Developer Guide

## Architecture Overview

BingDork Pro follows Clean Architecture principles with strict separation of concerns:

```
┌──────────────────────────────────────────────────┐
│                   CLI Layer                       │
│          (cobra commands, argument parsing)       │
├──────────────────────────────────────────────────┤
│                 Application Layer                 │
│    (orchestration, job management, scheduling)    │
├────────────┬────────────┬────────────┬───────────┤
│  Engine    │  Extractor │  Scheduler │  Output   │
│  (search   │  (data     │  (worker   │  (export) │
│   orchest) │   enrich)  │   pool)    │           │
├────────────┴────────────┴────────────┴───────────┤
│                 Domain Layer                      │
│   (core types, interfaces, domain logic)          │
├──────────────────────────────────────────────────┤
│              Infrastructure Layer                 │
│  (network, cache, storage, providers, metrics)    │
└──────────────────────────────────────────────────┘
```

### Key Design Decisions

1. **Interface-Driven Design** - All major components communicate through interfaces defined in `internal/core/types.go`
2. **Dependency Injection** - Components receive their dependencies through constructors
3. **Context Propagation** - Every operation accepts `context.Context` for cancellation and timeout
4. **Zero Global State** - All state is managed through explicit instances

## Adding a New Provider

1. Create a new package under `pkg/providers/<name>/`
2. Implement `core.SearchProvider` interface
3. Implement HTML parsing for the provider's search results
4. Add provider configuration to `core.ProviderConfig` and `core.ProvidersConfig`
5. Register the provider in `cli/cli.go` initializer

### Provider Example Template

```go
package myprovider

import (
    "context"
    "github.com/bingdork/bingdork/internal/core"
    "github.com/bingdork/bingdork/internal/logger"
    "github.com/bingdork/bingdork/internal/network"
)

type Provider struct {
    client *network.Client
    log    *logger.Logger
    cfg    *core.ProviderConfig
}

func New(cfg *core.ProviderConfig, netCfg *core.NetworkConfig, log *logger.Logger) (*Provider, error) {
    client, err := network.NewClient(netCfg, log)
    if err != nil {
        return nil, err
    }
    return &Provider{
        client: client,
        log:    log,
        cfg:    cfg,
    }, nil
}

func (p *Provider) ID() core.ProviderID { return "myprovider" }
// ... implement remaining interface methods
```

## Creating a Plugin

Plugins are Go shared libraries (.so files) loaded at runtime.

1. Implement the `plugin.Plugin` interface
2. Export a `Plugin` symbol from your shared library
3. Place the .so file in the plugins directory

```go
package main

import "github.com/bingdork/bingdork/internal/plugin"

type MyPlugin struct {
    plugin.BuiltinPlugin
}

var Plugin plugin.Plugin = &MyPlugin{
    BuiltinPlugin: plugin.BuiltinPlugin{
        Name:    "my-plugin",
        Version: "1.0.0",
    },
}

func init() {
    Plugin = &MyPlugin{}
}
```

Build with:
```bash
go build -buildmode=plugin -o my-plugin.so .
```

## Adding an Exporter

1. Implement `output.Exporter` interface
2. Add the exporter type to `NewExporterFromConfig()` factory
3. Register in CLI

## Testing

### Unit Tests

```bash
# Run all tests
go test -v -race -count=1 ./...

# Run specific package tests
go test -v ./pkg/parser/
go test -v ./internal/config/

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Profile CPU
go test -bench=. -cpuprofile=cpu.prof ./...
go tool pprof -pdf cpu.prof > cpu.pdf
```

### Fuzzing

```bash
go test -fuzz=. -fuzztime=60s ./...
```

## Code Style

- Follow standard Go conventions (gofumpt, goimports)
- Use `golangci-lint` with provided configuration
- All exported symbols must have documentation comments
- Keep interfaces small (1-3 methods preferred)
- Use `sync.Pool` for frequently allocated objects
- Avoid reflection unless absolutely necessary

## Performance Guidelines

- Use `strings.Builder` for string concatenation
- Pre-allocate slices when size is known
- Use streaming parsers for large result sets
- Implement `sync.Pool` for reusable buffers
- Profile before optimizing (use `pprof`)
- Avoid allocations in hot paths

## Release Process

1. Update version in `cli/cli.go`
2. Tag release: `git tag v1.0.0`
3. Push tag: `git push origin v1.0.0`
4. GitHub Actions builds and publishes release artifacts
5. Docker image is built and pushed to GHCR
