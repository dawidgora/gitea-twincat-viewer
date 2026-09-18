# Contributing

Thanks for contributing to gitea-twincat-viewer. Keep changes focused and
explain the user-facing or maintenance benefit in the pull request.

## Prerequisites

- Go 1.27
- Docker Compose, optional for integration work and the local Gitea stack

## Development workflow

Run the relevant Makefile targets before opening a pull request:

```bash
make build       # build ./bin/gitea-twincat-viewer
make test        # run the full test suite
make lint        # run gofmt checks and go vet
make dev         # start the local Gitea and viewer stack
make dev-down    # stop the local stack
make clean       # remove build artifacts
```

Docker Compose is required only for `make dev` and integration work. The
remaining targets use the Go toolchain.

## Contributions

- Follow the existing Go style and keep documentation concise.
- Add or update tests for behavior changes.
- Keep generated files and local build artifacts out of commits.
- Update user-facing documentation when behavior or configuration changes.

## Pull requests

Pull requests should include:

- A concise summary of the change and its motivation.
- The validation commands you ran and their results.
- Notes about any configuration, documentation, or compatibility impact.

Keep each pull request focused, respond to review feedback, and make sure the
branch is up to date before requesting review.
