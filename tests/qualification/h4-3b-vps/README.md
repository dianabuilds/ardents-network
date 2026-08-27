# H4-3B project-VPS Docker qualification

## Question

Do the exact current H4-3B/H4-6A Linux bytes retain their constrained Docker
result when executed on the declared project-operated VPS rather than the local
Docker engine?

## Run from Windows

```powershell
$env:ARDENTS_H4_3B_VPS = '203.0.113.7'
$env:ARDENTS_H4_3B_SSH_KEY = 'C:/absolute/path/to/id_ed25519'
make qualification-h4-3b-vps
```

The runner accepts only a literal IPv4 VPS address and a matching local private
key. It cross-builds the current Linux Endpoint, alpha-control, Node,
Reference-C2 fixture, Service test, and Endpoint-control test outside the
repository. It records SHA-256 values locally, uploads only those bytes into a
fresh exact `/tmp/ardents-h4-3b-vps-*` directory, and starts one disposable,
read-only Docker container per cell. Each container has no published port, no
external network, 1 vCPU, 1 GiB, 128 PIDs, and a private executable `/tmp`.

The cells are the four H4-3B HTTP/1.1 outcomes plus cached and two-fresh-root
H4-6A control observations. The runner records Docker version/image ID, kernel,
CPU count, and available memory, then removes the exact remote directory and
asserts it is absent. It never transfers the checkout, persistent keys, or
operator Docker configuration.

## Scope

This is one project-operated VPS Docker result. It is not a two-host C-2
qualification: Publisher and User remain processes within its one container.
It therefore does not establish an independent host, physical-host loss,
public-path failure/recovery, capacity, availability, a participant artifact,
or a selected browser/platform result. Missing SSH, Docker, image, cleanup, or
host prerequisites is an invalid selected environment and fails the target.
