# Installation

Текущая реализация Installation связывает проверенные файлы с локальной
конфигурацией, сохраняет состояние установки и замены и подготавливает передачу
управления программе. Это описание существующего кода, а не обоснование
самостоятельного продуктового домена или сохранения всех его механизмов.
Release определяет разрешённые файлы; работа запущенной программы и её данные
остаются у соответствующих владельцев.
Пакеты JOIN, Descriptor, Introduction и Forwarding не импортируют Source
Infrastructure; проверки артефактов и ошибки авторизации принадлежат Release.

Установка пользовательского приложения и развёртывание инфраструктуры —
разные сценарии. Платформы продукта включают Linux, macOS и Windows; конкретный
системный менеджер, структура путей и способ ограничения процесса не определяют
сам домен. Текущий установленный защищённый сценарий и его проверки ограничены
выбранным Linux-профилем. Переносимые проверки данных сами по себе не добавляют
поддержку запуска на других платформах.

[ADR-0125](../../../docs/adr/0125-separate-software-delivery-and-infrastructure-deployment.md)
убирает Installation как самостоятельный домен. Описанные ниже владельцы и
потребители остаются действующей реализацией до проверенного переноса; их
существование не означает завершения этого переноса.

Endpoint transactions/startup and Source transactions/startup have separate
owners and contracts. Manifest-pinned Node/Custody companions do not extend
Endpoint authority to Source or a Node role. Infrastructure implements Source's
initial stopped generation, guarded root/non-root startup, explicit stop/restart,
forward replacement and original-prefix recovery. Forwarding's distinct initial
stopped creator and canonical Node consumer retain their own account/birth/
selection lifetimes. Its separate root first-start producer and independently
opened non-root Startup retain distinct original custody and root archival/ACK;
explicit joined retirement and separately admitted same-generation restart
retain the complete original invocation chain. Canonical Forwarding also
implements explicit forward replacement of a proven stopped generation and
recovery of proven original prefixes to a stopped state. A replacement does
not start the selected generation. Installed acceptance applies to the
artifacts and scope recorded in the corresponding receipt. JOIN separately implements its canonical initial/start/stop/restart/replacement and safe-prefix recovery lifetimes. Descriptor separately owns protected root Start, independent non-root Startup, original joined Stop, retained restart, forward resource/program replacement and recovery of the proven complete stopped replacement prefix. Initial installation alone leaves all five semantic roots empty and grants neither Start nor Store initialization; Reachability remains the sole Store/proof/history owner. Introduction has distinct initial, protected first-start, independent Startup and history owners; its current Node contract defines the implemented lifecycle boundaries. Issuer installation is not supplied by those owners. Canonical Control consumers can
prepare Forwarding descriptor and initial-inventory bytes through Release
and Enrollment; public preparation supplies no Release or installed authority.
The [accepted infrastructure contract](../../../docs/technical/network-route-node.md#proposed-installed-infrastructure)
and [accepted ADR-0123](../../../docs/adr/0123-bind-installed-infrastructure-generations.md)
separately bind Source artifact admission, native effects and original startup.
Contract selection or a public receipt grants no install/start/recovery right;
in particular, an Endpoint Authorization or detached resource facts cannot be
reused as infrastructure permission.

| Пакет | Ответственность | Настоящие потребители |
|---|---|---|
| `release/endpoint` | Две свежие coherent Release authorizations для exact program/generation; независимая initial-pin provenance; private immutable inventory | `cmd/ardents-next/installation.go`, `endpoint/delivery` |
| [`endpoint/delivery`](../endpoint/delivery/README.md) | Request composition, bound generation, original lease/journal/resources, predecessor join, selection/reload, guarded startup и explicit recovery | `cmd/ardents-next/installation.go`, `cmd/ardents-next/installed_endpoint_linux.go` |
| [`infrastructure`](infrastructure/README.md) | Source request/template, native preflight, initial/replacement/recovery transactions, guarded startup and explicit stop/restart | Canonical `cmd/ardents-node/source_installation_linux.go`, `source_installed_linux.go`; public preparation in Control; temporary Next component consumers |
| `forwarding` | Fixed Forwarding request/plan/template, original input custody, initial stopped installation, guarded startup, joined stop, same-generation restart, explicit forward replacement and stopped-prefix recovery | Canonical Control preparation and Release signing; canonical Node preflight, installation, start/stop, replacement and recovery; Release retains the distinct artifact pair |
| `descriptor` | Fixed five-root declarations and original delivery; consumption of the genuine Release pair; stopped birth, protected startup, joined retirement/restart and forward replacement/complete-prefix recovery; no semantic authority or Store policy | Canonical Control public/initial/forward Release preparation; canonical Node install/inspect/start/stop/restart/replace/recover consumers |
| `join` | JOIN installation, guarded startup, original retirement, restart, replacement and recovery | Canonical Control and Node JOIN consumers |
| `introduction` | Introduction installation and original invocation lifecycle; separate slot history remains with Route | Canonical Control and Node Introduction consumers |
| `ubuntu` | Fixed Ubuntu infrastructure declaration and independent original file observation; startup admission stays with each caller | Source, Forwarding, JOIN, Descriptor and Introduction consumers |
| `nodeprincipal` | Fixed Node account rules, independent principal observation and original account creation custody; each role owns effect admission and journal | Forwarding, JOIN, Descriptor and Introduction account and principal observers |

`Authorization` остаётся opaque: соседний пакет не может заполнить private proofs,
pin или frozen bytes. `Targets` передаёт genuine Release proofs; `Descriptor`,
`Resource`, `Resources` и `InitialFacts` дают detached observations. Они не
создают installed ownership, право эффекта, rollback, Start или ACK replay.
`CoherentTargets` проверяет только agreement публичных фактов.

Логика разбора деклараций, проверки точных байтов и допустимых переходов не
зависит от ОС выполнения. Правила для устанавливаемой Linux-системы (например,
имена unit-файлов и предел пути Unix-сокета) могут проверяться переносимым кодом;
это не разрешает установку на другой ОС. Linux-адаптер отдельно владеет
системными вызовами, исходными дескрипторами и их фактическим закрытием.
Ошибки, идентичность исходного объекта и порядок допуск → эффект → проверка →
завершение сохраняются на границе между ними.

Директория роли не является группировкой для произвольных файлов: она должна
содержать правила и владельцев жизненного цикла именно этой роли. Общие
проверки записей и физические механизмы находятся у реального общего владельца;
каждый вызов сохраняет свои leases, журналы, полномочия и завершение. Разные
Source, Endpoint и Node-переходы не сводятся к общей разрешающей машине состояний.

Физические `directory`, `fixedfile`, `journal`,
`systemd`, `process`, `cgroup` и `completion` сохраняют собственные Interfaces,
Implementation и lifetimes. Их присутствие не даёт права установки или запуска.
Направление импортов задаётся exact package-map; nesting не разрешает обратный
импорт и не объединяет владельцев.

Endpoint contract и finite recovery grammar описаны у
[installed Endpoint](../endpoint/delivery/README.md) и в его
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

JOIN first start and non-root Startup are separate lifetimes in `join`. Root
admits current Release, completed initial/Hosting provenance, exact native
resources and one original Start/archive/ACK journal. Non-root Startup opens its
own immutable file/mount, process, namespace, read-only manager and original
root-peer exchange before genuine runtime construction. Unknown/retained attempts
stay blocking; kernel exit is no domain retirement. Source/Forwarding live owners
and permission never cross. Explicit JOIN retirement/restart/replacement/recovery
require their own independent contracts and verified consumers.

Introduction has separate initial and first-start transactions, independent
non-root Startup and read-only history owners in `introduction`, consumed
directly by canonical Node. Separate Infrastructure
Release admission and canonical fixed-purpose Control producers bind only the
Node/Introduction pair. Shared physical births and manager source transfer no live
owner or role permission. Initial installation leaves five semantic roots empty;
Route retains slot-history policy. Startup precedes genuine State/Node/Receiving
and Registry construction; initial completion alone grants none of those rights.
Unknown prefixes remain blocked. The current Node owner describes the distinct
startup and retirement boundaries; testing owns their native profiles, and the
selected issue owns acceptance. These components do not imply completed restart,
replacement/recovery or full installed qualification.
