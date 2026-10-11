# Installation

Домен сохраняет раздельные владельцы свежей авторизации и установленного Endpoint.

| Пакет | Ответственность | Настоящие потребители |
|---|---|---|
| `installation` | Две свежие coherent Release authorizations для exact program/generation; независимая initial-pin provenance; private immutable inventory | `cmd/ardents-next/installation.go`, `installation/endpoint` |
| [`endpoint`](endpoint/README.md) | Request composition, bound generation, original lease/journal/resources, predecessor join, selection/reload, guarded startup и explicit recovery | `cmd/ardents-next/installation.go`, `cmd/ardents-next/installed_endpoint_linux.go` |

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

Полный contract и finite recovery grammar описаны у
[installed Endpoint](endpoint/README.md) и в
[technical owner](../../../docs/technical/endpoint-service-runtime.md).
Перенос исходников и component checks не устанавливают complete interruption,
installed Service, power-loss или privacy qualification. Execution status и
приёмка остаются в выбранной GitHub issue.
