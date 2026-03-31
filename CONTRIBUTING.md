# Contributing to csm

## Development Setup

```bash
git clone https://github.com/ash0x0/csm.git
cd csm
make build
make test
```

Requires Go 1.24+ and [fzf](https://github.com/junegunn/fzf) for the interactive TUI.

## Code Style

- Run `go fmt ./...` before committing
- Run `go vet ./...` — must pass with zero warnings
- Follow standard Go conventions and naming

## Running Tests

```bash
make test          # run all tests with race detector
make cover         # generate HTML coverage report
```

## Pull Requests

1. Fork the repo and create your branch from `main`
2. Add tests for any new functionality
3. Ensure `make test` and `make vet` pass
4. Write a clear PR description explaining the change

## Reporting Issues

Open an issue on GitHub with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- `csm version` output
