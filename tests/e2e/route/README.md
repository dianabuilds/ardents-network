# Route process tests

These tests build `ardents-route`, start the Client, four Route positions, and
Publisher as separate operating-system processes, and assert the observable
authenticated canary and opaque-relay results. They retain no evidence bundle
and have no dependency on another test run.

Run them with `make e2e` or `go test ./tests/e2e/route`.
