# Stage 7 S7.0 start record

Status: **accepted; S7.0 complete and maintained S7.1 work authorized on
2026-08-20.**

## Bound predecessor and decision set

- Stage 6 disposition: `complete`; advance to Stage 7.
- Maintained-source baseline:
  `b8eb3b6ff8386c2b6166ca3a4ed35b04f75ac3bb`.
- Readiness checklist SHA-256:
  `24fcd10aefd67ad1af5381ee6f4465942a083343f91ce83592fe396fce270856`.
- ADR-0015 SHA-256:
  `f2b9e6b2fb0e330e2535dee4e47f466e90c7ff58f2a0fdfc0aaa610bc90ec684`.
- ADR-0016 SHA-256:
  `810df4f46498481022c8ac0156d40646b53045aeeb4ef022329a80066105136d`.
- ADR-0021 SHA-256:
  `a27f301dfe55976284e19bb4c0c4fa3a56f408f7bf9613b47ab04e24aa2a6bf7`.
- The commit containing this record is the S7.0 documentation/decision commit;
  no maintained Stage 7 implementation precedes it.

The accepted source set is the
[joint review](stage-7-joint-review.md),
[readiness checklist](stage-7-readiness-checklist.md),
[implementation brief](horizon-3-stage-7-brief.md),
[development plan](stage-7-development-plan.md),
[lifecycle specification](stage-7-lifecycle-spec.md),
[evidence contract](stage-7-platform-evidence.md), and
[development-host campaign specification](stage-7-host-campaign-spec.md), with
decided R-048–R-054/R-056 and accepted ADR-0015, ADR-0016, and ADR-0021.

## Product Owner authorization

```text
Product Owner authorization: start S7.1
Authorization date: 2026-08-20
Authorized first slice: S7.1 Release Decision
```

The Product Owner accepts that Ubuntu 26.04 Docker is not native Ubuntu Desktop
qualification, the current Windows 11 machine is not pristine, project-
controlled H3 inputs do not satisfy H4 independence, and pending/deferred
coverage is never a pass.

## Authorization boundary

This record authorizes maintained S7.1 behavior tests, Release Decision
implementation, the selected dependency closure, one non-test offline-import
caller, package-map registration when the package is real, and the S7.1 evidence
subset. Later slices follow the accepted development plan and their own evidence
gates.

It does **not** authorize MSI installation, repair, update, uninstall, purge,
installer-owned filesystem/Registry/service/startup/URI/browser/package
registration, reboot, destructive power/storage faults, or system DNS/route/
proxy/VPN/firewall mutation on Windows. Those operations still require the
Product Owner's separate command naming the exact artifact and permitted cells.
