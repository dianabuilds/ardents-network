# Active Testing

## Назначение

Каталог `docs/qa/active-testing/` описывает активные методы поиска слабых мест,
которые дополняют canonical test model, но не заменяют его.

Сюда входят методики, где инженер намеренно:

- меняет код;
- меняет runtime-параметры;
- вводит fault injection;
- ломает тайминги, connectivity или resource envelope;
- проверяет, как система обнаруживает, объясняет и переживает деградацию.

Этот каталог не является еще одним test layer. Это support layer для
exploratory weakness discovery. Любая значимая находка отсюда обязана
конвертироваться в canonical regression asset:

- scenario doc;
- `unit` / `integration` / `e2e` test;
- runner/reporting improvement;
- decision-log entry, если устранение пока невозможно.

## Состав каталога

- [Exploratory Testing](./exploratory-testing.md)
- [Mutation Testing](./mutation-testing.md)
- [Chaos Testing](./chaos-testing.md)
- [Fault Injection And Parameter Perturbation](./fault-injection-and-parameter-perturbation.md)

## Общий контракт

Любой active-testing experiment обязан заранее фиксировать:

- target domain или scenario;
- hypothesis о слабом месте;
- exact change, mutation или injected fault;
- expected detection signal;
- rollback path;
- blast radius;
- follow-up artifact в случае findings.

Active testing недопустим в форме:

- ad-hoc destructive activity без documented purpose;
- постоянного mutated product path;
- second runner или second source of truth;
- fake foundation вместо проверки реального runtime behavior.

## Связанные документы

- [QA Docs](../README.md)
- [Test Model](../test-model.md)
- [Integration Scenarios](../integration/README.md)
- [E2E Scenarios](../e2e/README.md)
