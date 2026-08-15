# Node process tests

These tests build the real Node command and exercise readiness, authenticated
role probes, duty replacement, drain, restart, credential isolation, and
cleanup through separate operating-system processes. Fixtures are generated in
the test temporary directory and disappear with the run.

Run them with `make e2e` or `go test ./tests/e2e/node`.
