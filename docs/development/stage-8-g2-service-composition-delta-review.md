# Stage 8 G2 Service and Endpoint composition delta review

Status: **S8.0 factual delta review; not a target-design decision.** This
temporary record rechecks G2 F034--F040 at the Stage 8 source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`.

## Method

No Stage-7-entry delta touches `serviceconn`, `serviceendpoint`,
`applicationipc`, `ardents-publish-app`, `ardents-service`, or
`ardents-stream-app`. The source therefore preserves the preparation findings
unless contradicted by direct current inspection.

The following source-bound diagnostics passed:

```text
go test ./internal/serviceconn ./internal/serviceendpoint \
  ./internal/applicationipc ./cmd/ardents-publish-app \
  ./cmd/ardents-service ./cmd/ardents-stream-app -count=1 -shuffle=on
```

`cmd/ardents-service` has no test files. Passing package tests do not create a
sequential-publisher, real Application Principal, current-policy, platform, or
claim Qualification receipt.

## Finding disposition

| Finding | S8.0 disposition |
|---|---|
| F034 | **Confirmed; open.** Current `serviceconn.Request`, `endpoint.Do`, and Result remain action/result unions that expose unrelated publication, key, route, session, and evidence facts to callers. |
| F035 | **Confirmed; open.** One accepted connection can retire current publication; the current fixed batch is a tracer workaround, not a publisher lifecycle. |
| F036 | **Confirmed; open.** Static plan time and public recovery facts remain the authority source for later work; no authenticated current-policy refresh is present. |
| F037 | **Confirmed; open.** Current exclusive-generation persistence is not a crash-atomic publication lifecycle. |
| F038 | **Confirmed; open.** Fixed stream byte counts, 16 accepts, plan/socket compatibility, and the result socket are current tracer controls, not a product live-stream contract. |
| F039 | **Confirmed; open.** Application result and process/evidence observations remain one mutable result surface. |
| F040 | **Confirmed; open.** Current tests construct staged fields and omit sequential publication, current-time/revocation, crash lifecycle, capacity, and drain rows; no blind duplicate deletion is justified. |

## Consequence for Stage 8

If S8.1 preserves a Service product, S8.3 must establish distinct
`application/broker`, `service/connection`, `service/publication`, and
composition responsibilities before moving packages. Connection must receive
opaque verified current intent/policy rather than caller-built authority bags;
Publication must own acquisition/drain/generation state; endpoint process
evidence must not expand the Application result contract. The current tracer
may be retained only as characterization until its evidence is deliberately
replaced or retired.

This record makes no product-disposition decision.
