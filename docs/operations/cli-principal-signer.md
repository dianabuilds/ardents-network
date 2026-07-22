# CLI Principal Signer Operations

`ardentsctl identity` manages local Principal and device signer custody. These
commands are deliberately offline: they do not contact a Node, consume a
Bootstrap Ticket, create a session, or grant authority.

## Create And Inspect A Principal

Create a new root signer at the protected default location:

```text
ardentsctl identity principal create
ardentsctl identity principal show
```

Use `--signer-file PATH` to choose an explicit location. Creation is atomic and
refuses any existing destination. To copy an already protected Ardents root
bundle into a new no-replace destination, use:

```text
ardentsctl identity principal import --from-file SOURCE --signer-file DESTINATION
```

Import does not accept an arbitrary raw seed. It first checks the source file's
private-file protection and then verifies the complete version, algorithm,
public/private key, and self-certifying Principal binding.

## Create And Inspect A Routine Device

Create a separate device key and root-signed authenticate-only Credential:

```text
ardentsctl identity device create
ardentsctl identity device show
```

The defaults read
`<os.UserConfigDir>/ardents/identity/principal-root-v1.json` and create
`<os.UserConfigDir>/ardents/identity/device-v1.json`. Override them with
`--root-signer-file` and `--signer-file`. `--valid-for` defaults to 90 days and
cannot exceed 365 days.

After creating the device bundle, move the root bundle to the intended offline
custody boundary. Normal authentication uses only the device bundle. Do not
copy either file into Node state, logs, tickets, issue reports, or diagnostic
snapshots.

## Protection And Output

On Unix, signer directories are restricted to `0700` and signer files to
`0600`; a permissive existing signer is rejected rather than silently repaired.
On Windows, Ardents applies a protected DACL for the current user and SYSTEM.
Creation and import never replace a destination, including under concurrent
execution. An existing parent directory is checked but never chmod/ACL-rewritten;
choose a dedicated private signer directory if an explicit path's parent is
shared. Temporary signer files are protected before key bytes are written.

Human and `--output json` display only the Principal, DeviceID, public keys,
Credential ID, and validity. They never display private seeds, Credential wire
bytes, signatures, or session material. Unknown fields, duplicate JSON keys,
noncanonical base64url, corrupt Credentials, and key/identifier mismatches fail
closed.

Enrollment, grant administration, device revocation, and Principal sessions are
documented in `docs/operations/cli-principal-sessions.md`.
