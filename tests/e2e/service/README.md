# Service command end to end

These tests build the Service and Application commands and exercise their
public Unix-socket boundaries. They cover readiness timeout and cleanup plus an
Exact-Target stream that retains one Service Connection when its current Route
fails and the first replacement also fails. The recovery scenario mocks only
the external Route socket; Service, publication, Application IPC, continuity,
and workload processes are real commands.
