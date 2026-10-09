# Service Publication

Go-пакет проверяет публичные Credential и Publication v3: подписи Authority и
Instance, точный Target/Network, validity и отдельные права чтения и публикации.
Отдельный `durable` владеет исключительным v3 root, canonical floor и
восстановлением подписанного public record. Настоящая команда
`ardents-next publication prepare-root` возвращает проверенную public history;
она не восстанавливает private Instance или accepting readiness. Живой Publisher,
refresh/withdraw и приватный получатель здесь ещё не реализованы.

**Вопрос домена:** какой авторизованный Service Instance сейчас принимает работу?

**Входит:** поколение и материал Instance по назначению, ревизия публикации,
текущая/подготавливаемая/предыдущая пара регистраций, приватный получатель,
защита от повторов, готовность, обновление и отзыв публикации.

**Не входит:** корневая подпись Credential, транспорт Route, история конфликтов
Descriptor Store, восстановление Connection, изменение Job и очистка worker.

**Состояние и завершение:** перестать принимать новые операции, завершить свои
регистрации и получателей, стереть приватный материал. Поздний ACK не возрождает
поколение. ACK регистрации и ACK Descriptor Store — разные основания готовности.

**Соседи:** Execution предоставляет точную живую операцию; Route — регистрацию
и непрозрачную доставку; Reachability — размещение Descriptor. Custody сохраняет
корневые полномочия; runtime-ключ Instance не превращается в корневую власть.

**Где искать обязанности:** `internal/service/instance`, `service/publication`,
регистрация и Introduction в `internal/endpoint`. Переносить только обязанности
публикации: принимающие слоты и их история принадлежат Route.

**Основа:** [Service](../../../docs/technical/endpoint-service-runtime.md),
[reachability](../../../docs/technical/private-reachability.md),
[карта доменов](../../../docs/development/domain-map.md).
