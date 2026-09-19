# R-156 — Hosting accounting and recovery characterization

Baseline: 6a07fd9b1aaea7b74f929cefede79b9446c2defc (2026-09-14).
Question: what useful capacity and recovery remain after measured consumption,
loss of a reservation handle, and a host reboot?

Predeclared observations:
- A full reservation remains held while the observed counter increases.
- With allowance 1000 B, reservation 700 B and observed usage 300 B, the current
  model requests drain. This is conservative accounting, not a claim that the
  provider allowance has actually been spent.
- A new boot identity refuses without erasing the old consumed floor; a retry
  alone cannot repair continuity.
- The persistent reservation aggregate contains neither individual identities
  nor deadlines, so observation cannot classify abandoned individual work.

Run a clean temporary extraction of the baseline, copy
hosting_model_test.go.txt into internal/resource/r156_hosting_model_test.go,
then execute:
```text
go test ./internal/resource -run '^TestR156Hosting' -v -count=1 -timeout=30s
```

These portable tests execute the actual accounting/observation model with
declared synthetic readings. They do not execute Linux flock, sysfs, fsync,
Node shutdown or an installed machine reboot. The fixture's small byte counts
are arithmetic inputs, not legal forwarding envelopes. No production files or
new maintained profiles are introduced. Outputs belong outside the repository.

Result: both characterization tests passed on 2026-09-14 (exit 0), Go 1.26.8
windows/amd64, GOPROXY=off and GOSUMDB=off, using cached dependencies and a
clean temporary extraction of the baseline. This PASS confirms the described
conservative behavior; it does not accept the recovery design for production.

Observed output:
- used=300 reserved=700 remaining=0 drain=true
- new boot refused; retry remains refused; old consumed floor preserved

Raw output is outside Git:
C:\Users\vitek\AppData\Local\Temp\ardents-r156-hosting-e18a56bf98f44834b5c49f9e2146e686\snapshot\r156-hosting-output.txt
SHA256: a4c084359d183963b32d82675fd638af9bdea60655e11a95d59ffcb9dbd99e12

Preserve this as research characterization, not as a replacement for installed
recovery and provider-accounting acceptance. No Linux execution, race run,
full make check or installed host qualification is claimed.
