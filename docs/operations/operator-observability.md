# Operator Observability

Ardents exposes production monitoring on a dedicated read-only HTTP listener.
The default address is `127.0.0.1:9090`.

## Endpoints

- `GET /healthz`: process liveness; HTTP 200 while the listener serves.
- `GET /readyz`: canonical lifecycle plus Diagnostics readiness; HTTP 200 only
  for a ready, healthy node and HTTP 503 otherwise.
- `GET /metrics`: Prometheus/OpenMetrics exposition.

Each response carries `Ardents-Correlation-ID`. Operators may provide a safe
1-64 character ID using letters, digits, `.`, `_`, or `-`.

## Remote Scraping

The daemon rejects non-loopback `observability.listen_address` values. Remote
scraping must pass through a deployment-owned authenticated TLS proxy, sidecar,
or equivalent private transport that connects to the loopback listener. A
configured `observability.token_file` must be regular and private to the daemon
user. It contains a dedicated bearer token, not the control API token.

Prometheus scrape configuration shape:

```yaml
scrape_configs:
  - job_name: ardents
    authorization:
      credentials_file: /run/secrets/ardents-metrics-token
    static_configs:
      - targets: ["ardents-observability-proxy:443"]
```

The proxy owns TLS server identity, client authentication, and deployment
network policy. Direct daemon exposure is not supported. Health/readiness are
intentionally minimal and do not require the scrape credential.

## Versioned Artifacts

- `deploy/docker/observability/prometheus-alerts.yml`
- `deploy/docker/observability/grafana-dashboard.json`

The dashboard and alerts use only bounded aggregate labels. Diagnostics RPCs
remain the source for resource-specific explanations and recovery actions.
