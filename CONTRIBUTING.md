# Contributing

## Issues

Report bugs or suggest features by [opening an issue](https://github.com/st-shivam/kblog/issues/new).

Security vulnerabilities should be reported privately to **stripathidev@gmail.com** — see [SECURITY.md](SECURITY.md).

## Development setup

Requires Go 1.26+.

```bash
git clone https://github.com/st-shivam/kblog.git
cd kblog
make build
```

## Running tests

```bash
# Unit tests
make test

# Unit tests with race detection (CI)
go test -v -race ./...

# Integration tests (requires k3d-dev cluster)
./tests/run_tests.sh --context k3d-dev
```

## Code style

- Format: `gofmt` (no formatter config needed)
- Lint: `go vet ./...` (no golangci-lint)
- Tests: standard `testing` package only (no testify, no Ginkgo)
- No codegen, no pre-commit hooks

## Pull requests

CI runs on every PR to `main`:

1. `go mod verify`
2. `go vet ./...`
3. `go build -v ./...`
4. `go test -v -race ./...`

Make sure all steps pass before requesting review.
