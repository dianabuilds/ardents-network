# MR-06 findings detail — run 1

There are no Medium-or-higher findings remaining at target commit `d05fa68`.

The one Medium pre-remediation candidate was independently confirmed and
remediated within the audit:

```text
stable certificate_ref
  -> new cert/key/CA material behind the reference
  -> pre-remediation configuration digest unchanged
  -> host reports unchanged; no restart
  -> old in-memory WSS credential and reachability generation remain ready

remediation
  -> protected validated certificate-material generation digest changes
  -> affected host configuration digest changes
  -> only affected host must report restarted
  -> new process starts with AutoNAT Unknown
  -> final protected status must bind the new running generation
```

Independent Phase 6 reviewers verified the remediation and found no new bypass
or ordinary-output leak.
