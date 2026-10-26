# Installation

Домен сохраняет раздельные владельцы свежей авторизации и установленного поколения.

Endpoint transactions/startup and Source transactions/startup have separate
owners and contracts. Manifest-pinned Node/Custody companions do not extend
Endpoint authority to Source or a Node role. Infrastructure implements Source's
initial stopped generation, guarded root/non-root startup, explicit stop/restart,
forward replacement and original-prefix recovery. The five Node roles have no
accepting installed transaction here.
The [accepted infrastructure contract](../../../docs/technical/network-route-node.md#proposed-installed-infrastructure)
and [accepted ADR-0123](../../../docs/adr/0123-bind-installed-infrastructure-generations.md)
separately bind Source artifact admission, native effects and original startup.
Contract selection or a public receipt grants no install/start/recovery right;
in particular, an Endpoint Authorization or detached resource facts cannot be
reused as infrastructure permission.

| Пакет | Ответственность | Настоящие потребители |
|---|---|---|
| `installation` | Две свежие coherent Release authorizations для exact program/generation; независимая initial-pin provenance; private immutable inventory | `cmd/ardents-next/installation.go`, `installation/endpoint` |
| [`endpoint`](endpoint/README.md) | Request composition, bound generation, original lease/journal/resources, predecessor join, selection/reload, guarded startup и explicit recovery | `cmd/ardents-next/installation.go`, `cmd/ardents-next/installed_endpoint_linux.go` |
| [`infrastructure`](infrastructure/README.md) | Source artifact admission, closed request/template, native preflight, initial/replacement/recovery transactions, guarded startup and explicit stop/restart | Canonical `cmd/ardents-node/source_installation_linux.go`, `source_installed_linux.go`; public preparation in Control; temporary Next component consumers |

`Authorization` остаётся opaque: соседний пакет не может заполнить private proofs,
pin или frozen bytes. `Targets` передаёт genuine Release proofs; `Descriptor`,
`Resource`, `Resources` и `InitialFacts` дают detached observations. Они не
создают installed ownership, право эффекта, rollback, Start или ACK replay.
`CoherentTargets` проверяет только agreement публичных фактов.

Физические `request`, `directory`, `generation`, `fixedfile`, `journal`, `unit`,
`systemd`, `process`, `cgroup` и `completion` сохраняют собственные Interfaces,
Implementation и lifetimes. Их присутствие не даёт права установки или запуска.
Направление импортов задаётся exact package-map; nesting не разрешает обратный
импорт и не объединяет владельцев.

Endpoint contract и finite recovery grammar описаны у
[installed Endpoint](endpoint/README.md) и в его
[technical owner](../../../docs/technical/endpoint-service-runtime.md).
Source сохраняет отдельные grammar и authority у
[Infrastructure](infrastructure/README.md) и своего
[lifecycle owner](../../../docs/technical/network-route-node.md#initial-stopped-source-persistence).
Перенос исходников и component checks не устанавливают complete interruption,
installed Service, power-loss или privacy qualification. Execution status и
приёмка остаются в выбранной GitHub issue.
Installed Source execution additionally requires the exact public bootstrap
receipt's direct PO acceptance; implemented composition and component checks do
not supply that decision or installed qualification. Source explicit stop/restart
separately consumes complete original retirement and fresh invocation admission;
installed qualification remains separately gated. Forward replacement and
explicit recovery acquire their own fresh Release/native/journal admission.
Recovery may finish only proven original prefixes; it never replays startup,
ACK or retirement, adopts foreign resources or resets semantic histories.
Unknown or uncertain original completion remains repair-required.
