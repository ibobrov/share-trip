# Тестирование проверки договора при создании и публикации заявки

## Unit-тесты сервисного слоя

`internal/service/create_trip_test.go` проверяет решения `CreateTrip` без сети и PostgreSQL.
Зависимости заменены mock, сгенерированными `mockgen` из `go.uber.org/mock` версии `v0.6.0`.

- Разрешение: правильные компания и `trip_creation`, затем `BEGIN`, доменный usecase и `COMMIT`.
- Бизнес-отказ: `ErrForbidden`, причина отказа, ноль вызовов `BEGIN` и доменного usecase.
- Timeout, отмена и ошибка клиента: ошибка сохранена в цепочке, транзакция не начинается.
- Ошибка `BEGIN`: домен не вызывается.
- Ошибка домена: `ROLLBACK` без `COMMIT`.
- Ошибка `COMMIT`: вызывающий код получает ошибку вместо созданной заявки.

`gomock.InOrder` фиксирует порядок, `Times(1)` — единственный вызов,
`Times(0)` запрещает вызов. На уровне сервиса клиент вызывается один раз:
повтор HTTP 500 выполняется внутри HTTP-адаптера.

`tx` всегда вызывает отложенный `Rollback`. После успешного `Commit` это безопасный
вызов, возвращающий `pgx.ErrTxClosed`; unit-тест учитывает это поведение.

`internal/service/metrics_test.go` проверяет метрики по итоговой ошибке метода.

## Публикация поездки

`internal/service/publish_trip_test.go` проверяет отдельное чтение без транзакции
до обращения в Contract Service и начало транзакции изменения только после разрешения.
Проверяются компания сохранённой поездки, код `trip_publication`, отказ, timeout,
проверка владельца, повторная публикация и отказ домена при изменении статуса
во время сетевой проверки. При отказе или ошибке проверки нет вызовов изменения
поездки и формирования outbox-события.

`internal/api_test/publish_trip_contract_test.go` проверяет разрешение, истечение
договора и отключение услуги после создания черновика, ошибку Contract Service,
сохранность поездки, истории и outbox при отказе. Подмена компании в текущем токене
не меняет компанию, отправляемую на проверку. Повторная публикация успешна даже
при недоступном Contract Service и не обращается к нему.
Отдельный тест держит блокировку строки в другой транзакции и проверяет, что
предварительное чтение и отказ Contract Service выполняются без ожидания этой блокировки.

## HTTP-адаптер

`internal/clients/contract/client_test.go` использует настоящий клиент и локальный
`httptest.Server`. Проверяются метод, URL, JSON, заголовок Content-Type, ответы,
ошибки, повтор HTTP 500, timeout и отмена контекста. Contract Service не запускается.

## Интеграционные тесты

`internal/api_test/create_trip_contract_test.go` выполняет запрос через API,
настоящий HTTP-клиент и тестовый HTTP-сервер Contract Service. PostgreSQL запускается
в Docker. Проверяются HTTP-ответы и фактические записи поездки и истории.

Отсутствие записей в БД и отсутствие начала транзакции — разные проверки.
Последнее отдельно проверяется unit-тестами сервисного слоя.

## Kafka producer

`internal/events/producer_test.go` проверяет JSON, ключ `trip_id` и передачу ошибок.
`internal/events/producer_integration_test.go` проверяет отправку и чтение в реальной
Kafka только при заданном `KAFKA_TEST_BROKERS`, в отдельном временном topic.
API-тесты проверяют pending-событие в БД и отсутствие отправки в пользовательском запросе.
`internal/api_test/outbox_publisher_test.go` проверяет отправку, повтор после сбоя,
десять ошибок, конкурирующих publisher и повтор после ACK при сбое отметки sent.
`internal/api_test/outbox_kafka_test.go` проверяет восстановление доставки из PostgreSQL
в настоящую Kafka после недоступности брокера; требует `KAFKA_TEST_BROKERS`.
Service-тесты проверяют единую транзакцию поездки, истории и события.
Настройки, ограничения доставки и диаграмма описаны в [документе producer](kafka-producer.md).

## Команды

Кодогенерация:

| Репозиторий | Команда | Результат |
| --- | --- | --- |
| share-trip-contract | `make generate` | Типы и серверный интерфейс из `api/contract.yaml`, oapi-codegen v2.8.0 |
| share-trip | `make generate` или `make generate-mocks` | Mock сервисных зависимостей, mockgen v0.6.0 |
| share-trip | `make generate-contract-client` | HTTP-клиент из `http://localhost:38082/api/contract.yaml`, oapi-codegen v2.8.0 |
| share-trip-notification | Нет | Кодогенерация не настроена |

`make k8s-up` собирает уже существующий сгенерированный Go-код и не запускает генераторы.
После изменения контракта выполните `make generate` в share-trip-contract, затем
`make k8s-up` в share-trip, чтобы работающий Contract отдавал обновлённый OpenAPI.
После этого выполните `make generate-contract-client` в share-trip. Если изменился код клиента,
для его использования в pod-е нужен повторный `make k8s-up`.
Генерация клиента также выполняет `go mod tidy`.

```text
make generate-mocks
go test ./internal/service
go test ./internal/clients/contract
go test ./...
```

`make generate-mocks` устанавливает генератор в `.tools`, если он отсутствует,
и запускает директивы `go:generate` через переменную `MOCKGEN`, которую экспортирует
Makefile. `make generate` также генерирует mock. Сгенерированные файлы находятся
в `internal/service/mocks`; их сохраняют вместе с исходниками и не редактируют вручную.

Для полного набора тестов нужен работающий Docker. Unit-тестам сервиса и тестам
HTTP-клиента Docker не требуется.

PostgreSQL тестов создаётся Testcontainers отдельно от БД стенда; Docker Compose и Kubernetes
для этих тестов не требуются. Kafka-тесты с `KAFKA_TEST_BROKERS` используют отдельно доступный
брокер, как описано в [kafka-producer.md](kafka-producer.md).
