# Service Connection

Начальная ограниченная реализация связывает проверенную Publication и поля
приватной капсулы с исходной квалифицированной Execution operation. Она вычисляет
неизменяемый logical context и отдельный Attachment context, отказывает при
подмене Target/Network/profile и расширении исходных Work Safety пределов.
Publication runtime потребляет эту привязку после собственных проверок реальной
регистрации, Instance и независимо разрешённого Rendezvous. Привязка не является
успешным Service Connection: Instance TLS, continuity, ordered bytes, recovery
и терминальный исход полного потока ещё не реализованы этим владельцем.

## Interface и Implementation

`BindRecipient` требует исходную живую Publisher operation и sealed Publication
proof. `RecipientBinding.Check` проверяет ту же operation и исходный capsule
deadline перед первой передачей результата; `Contexts` возвращает только копии
transcript commitments. `recipient_binding.go` хранит эти неизменяемые факты
без сетевых эффектов, приватного ключа или отдельного cleanup ресурса.
Поведение проверяется независимыми signed-input/transcript fixtures и отказами
при подмене, расширении пределов и отсутствии исходной operation. Эти fixtures
не квалифицируют worker, принимающий Publisher или Service stream.

**Вопрос домена:** как один аутентифицированный логический поток сохраняет идентичность?

**Входит:** неизменные Target и происхождение доказательства, точная
аутентификация Instance, упорядоченные байты, поколения Attachment,
исходный предел восстановления и сохранённый терминальный результат.

**Не входит:** повтор Application-операции, новый Target или Grant, изменение
Job, очистка worker, публикация, политика пути и бюджет Hosting.

**Состояние и завершение:** прекращает работу и дожидается своих Attachment,
сохраняет логический исход. Восстановление заменяет Attachment, но не Target,
Job, Grant или исходный срок восстановления. Route завершает физический поток,
Execution — worker; эти обязанности не переходят к Connection.

**Соседи:** Reachability предоставляет проверенное доказательство; Route —
новую физическую попытку в исходных пределах; Execution — точную живую операцию,
проверяемую перед эффектами и окончательной передачей результата.

**Где искать обязанности:** `internal/service/connection`, Service binding
и восстановление в `internal/endpoint`, `internal/endpoint/service`.
Механизмы и полномочия разбираются отдельно; старые callbacks не переносятся
как междоменный контракт.

**Основа:** [Service](../../../docs/technical/endpoint-service-runtime.md),
[граница Execution](../../../docs/development/application-execution-domain-boundary-analysis.md),
[карта доменов](../../../docs/development/domain-map.md).
