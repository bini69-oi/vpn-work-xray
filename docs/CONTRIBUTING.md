# Contributing

## How to contribute

1. Fork the repository.
2. Create a branch: `git checkout -b feature/my-feature`
3. Make changes.
4. Ensure checks pass: `make verify` (or at least `make verify-quick` during iteration).
5. Commit: `git commit -m "feat: short description"`
6. Push: `git push origin feature/my-feature`
7. Open a Pull Request.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — new feature
- `fix:` — bug fix
- `docs:` — documentation only
- `refactor:` — refactor without behavior change
- `test:` — tests
- `chore:` — tooling, CI, dependencies

## Code style

- Go: `golangci-lint` (see `.golangci.yml`)
- Python (telegram-bot): `black` + `ruff` (recommended)
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Target coverage for product packages: see `Makefile` (`MIN_COVERAGE`, default 80% for selected packages in `verify`)
