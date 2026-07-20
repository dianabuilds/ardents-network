# Mutation Testing

## Суть

Mutation testing проверяет, способны ли текущие automated tests и diagnostics
обнаружить намеренно внесенную поломку product behavior.

Метод не отвечает на вопрос "есть ли у нас тесты вообще". Он отвечает на вопрос
"ловят ли наши тесты и проверки реальную деградацию нужного инварианта".

## Что разрешено мутировать

Допустимые mutation targets:

- ветвления и guard conditions;
- error handling и retry decisions;
- policy checks;
- publication eligibility;
- recovery transitions;
- assertions в tests;
- freshness, timeout и backoff logic;
- state transitions, если mutation не создает неснимаемый side effect.

Mutation должна быть привязана к конкретному product invariant, а не к
абстрактному "посмотрим, что будет".

## Базовый процесс

1. Выбрать invariant, scenario step или degraded path.
2. Сформулировать hypothesis: какая поломка должна быть поймана и чем именно.
3. Внести ограниченную mutation в локальную ветку или временный рабочий diff.
4. Запустить релевантный canonical test path.
5. Сравнить фактический detection signal с ожидаемым.
6. Откатить mutation полностью.
7. Если mutation survived, создать formal follow-up artifact.

## Методики проверки

### Branch And Condition Mutation

Подмена `if`/`switch` условий, инверсия guard logic, удаление rejection path.

Проверяет:

- что tests ловят недопустимый success path;
- что policy и validation не декоративны;
- что degraded/failure assertions реально существуют.

### Error-Path Mutation

Удаление error return, подмена fallback path, suppress логики propagation.

Проверяет:

- что сломанный failure path не проходит как success;
- что diagnostics не теряют explainability;
- что runtime не скрывает broken state.

### Assertion Mutation

Ослабление или удаление assertions в tests, чтобы понять, не проходят ли suites
за счет слишком слабого observable contract.

Проверяет:

- false positive risk;
- наличие реального observable outcome;
- не держится ли тест на incidental behavior.

### Timing And Threshold Mutation

Изменение timeout, retry count, freshness window, debounce/backoff thresholds.

Проверяет:

- чувствительность сценария к реальным runtime границам;
- не замаскирована ли flaky behavior слишком широкими допусками;
- есть ли покрытие для degraded timing paths.

## Критерии качества

Mutation testing считается полезным только если:

- у mutation был заранее известный expected signal;
- experiment был reversible;
- surviving mutation породила конкретное remediation action;
- findings можно оттрассировать к scenario doc, test или decision trail.

## Недопустимые практики

- оставлять mutated code в основной product path;
- считать manual observation достаточным evidence без formal follow-up;
- использовать mutation как замену normal regression testing;
- мутировать foundation так, что тест уже не проверяет продукт, а только шум от окружения.
