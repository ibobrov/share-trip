# Наблюдаемость публикации поездки

## Идентификаторы

| Поле | Назначение |
| --- | --- |
| `request_id` | Один входящий HTTP-запрос; `X-Request-ID`, генерируется UUID при отсутствии |
| `correlation_id` | Весь процесс; `X-Correlation-ID`, по умолчанию равен request_id |
| `causation_id` | Для TripPublished — request_id HTTP-команды PublishTrip |
| `event_id` | UUID outbox-события, сохраняется при повторной отправке; ключ inbox |
| `trace_id` | Трасса OpenTelemetry; передаётся через W3C traceparent, а не отдельный trace_id header |
| `trip_id`, `client_id` | Поездка и инициатор; остаются в логах и trace attributes, не в labels метрик |

ShareTrip передаёт в Contract `X-Request-ID`, `X-Correlation-ID`, `traceparent`,
а также диагностические `X-Trip-ID` и `X-User-ID`. Последние не используются Contract
для авторизации. Компания по-прежнему проверяется из тела CheckOption.
Итоговый API-лог проверки прав содержит `service=contract`, `component=ContractServer`,
`operation=CheckPermission`, идентификаторы и result.

Metadata сохраняется в существующем `outbox_event.payload` вместе с бизнес-полями;
новые столбцы и SQL-миграция не нужны. Пример сохранённого JSON:

```json
{
  "trip_id": "f9b711e3-8f45-4dc3-8cab-07623f5de6e5",
  "client_id": "a1b2c3d4-1111-4111-8111-123456789abc",
  "company_id": "company-acme",
  "request_id": "request-123",
  "correlation_id": "publication-456",
  "causation_id": "request-123",
  "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
}
```

`event_id` и `occurred_at` берутся из `outbox_event.id` и `created_at`.
Producer добавляет Kafka headers `event_id`, `event_type`, `request_id`, `correlation_id`,
`causation_id`, `traceparent`, `tracestate`. Передаваемый traceparent указывает на span
текущей попытки публикации; сохранённый в outbox контекст остаётся исходным.
При повторах event_id, correlation_id и causation_id не меняются.

Consumer создаёт новую трассу со span link на контекст Kafka. Поэтому trace_id consumer-а
отличается от trace_id HTTP-команды: связывайте их через span link, correlation_id и event_id.
Offset подтверждается после COMMIT inbox и уведомления. При ошибке commit offset consumer
останавливается; повторное чтение безопасно благодаря inbox.

Старые outbox-записи и Kafka-сообщения без metadata поддерживаются. Для них correlation_id
при отправке берётся из event_id; восстановить прежнюю HTTP-трассу невозможно.
Headers event_id/event_type, если присутствуют, должны совпадать с payload.

## Метрики

| Метрика | Значение |
| --- | --- |
| `sharetrip_trip_publish_total{result}` | Публикации, включая повторные и отклонённые; сохранено существующее имя |
| `sharetrip_trip_publish_duration_seconds{result}` | Длительность сервисной операции публикации |
| `sharetrip_contract_request_total{result}` | Проверки Contract: allowed, denied, error; HTTP retries считаются одной проверкой |
| `sharetrip_contract_request_duration_seconds{result}` | Проверка Contract вместе с повторами и задержками |
| `contract_permission_check_total{result}` | Проверки на стороне Contract; каждая HTTP-попытка отдельно |
| `contract_permission_check_duration_seconds{result}` | Длительность проверки Contract |
| `contract_http_requests_total{method,route,status}` | HTTP-запросы Contract |
| `sharetrip_outbox_pending` | Gauge: число pending-записей по последнему успешному отдельному опросу |
| `sharetrip_outbox_publish_total{result}` | Попытки публикации; success означает ACK Kafka, но не COMMIT отметки sent |
| `sharetrip_outbox_publish_failed_total` | Ошибки попыток отправки, включая повторные; это не число строк со статусом failed |
| `notification_consume_total{result}` | success, duplicate, ignored, error; успешные результаты учитываются после commit offset |
| `notification_consume_duration_seconds{result}` | Обработка сообщения вместе с commit offset |
| `notification_inbox_duplicate_total` | Дубликаты, отсечённые успешно зафиксированной inbox-транзакцией |
| `notification_persist_total{result}` | Результат сохранения уведомления TripPublished: created либо error |
| `notification_persist_duration_seconds{result}` | Длительность транзакции сохранения уведомления TripPublished |

Внешний SMS/email-провайдер не подключён. Метрики `notification_persist_*`
измеряют сохранение, а не доставку пользователю.
Inbox-дубликат не увеличивает число созданных уведомлений.

Kafka Exporter публикует `kafka_consumergroup_lag`, `kafka_consumergroup_members`
и `kafka_topic_partitions`. Это реальные имена метрик выбранного
[exporter](https://github.com/danielqsj/kafka_exporter#metrics).
Он использует совместимый протокол Kafka 3.6 при подключении к локальному брокеру 4.1.
До появления consumer group часть рядов может отсутствовать. Lag `-1` у exporter-а означает
отсутствие подтверждённого offset. Для такой partition dashboard вычисляет backlog как latest minus oldest:
consumer настроен начинать с FirstOffset. Отсутствующие данные не подменяются нулём.

При изменении topic или consumer group согласуйте фильтры Kafka Exporter и запросы dashboard.

Идентификаторы поездок, событий и пользователей не используются как labels.
Метрики доступны без токена на `/metrics` всех трёх приложений; Prometheus опрашивает их
и Kafka Exporter каждые 10 секунд. Конфигурации лежат в `k8s/config`.

## Dashboard Trip publication

После `make k8s-up` откройте
[Grafana → Trip publication](http://localhost:33000/d/trip-publication).
Dashboard находится в папке ShareTrip и создаётся через provisioning из
[trip-publication.json](../k8s/config/trip-publication.json).
Выберите источник Prometheus, если Grafana не выбрала его автоматически.

Панели показывают публикации, долю ошибок, p95 HTTP и Contract, outbox backlog,
ошибки Kafka publish, consumer lag и участников группы, inbox-дубликаты и ошибки
сохранения уведомлений. Названия метрик и ограничения описаны выше.
Без новых запросов p95 и rate могут не иметь данных; после запуска требуется минимум два scrape.
Недоступный scrape отображается отдельно: отсутствие данных не означает отсутствие ошибок.

Изменяйте исходный JSON в репозитории: редактирование provisioned dashboard через UI
может быть заменено следующей загрузкой конфигурации. `k8s-up` перезапускает Grafana
и Prometheus для перечитывания их ConfigMap-файлов.

## Как расследовать: уведомление не пришло

1. Найдите запрос PublishTrip по `request_id` или `trip_id` в Loki. Скопируйте `correlation_id`.
2. Найдите CheckPermission в Contract с тем же correlation_id: `allowed`, `denied` или `error`.
3. Проверьте состояние поездки и строку outbox. HTTP 201 подтверждает COMMIT публикации,
   а не доставку уведомления.
4. По event_id проверьте status, attempts, last_error, sent_at. `pending` означает ожидание
   отправки; `failed` после лимита попыток требует разбора. ACK без COMMIT sent может дать дубль.
5. Проверьте consumer lag в dashboard или Kafka UI для `trip.events`, group `notification-service`.
6. В логах Notification найдите ConsumeTripPublished и InboxInsert по event_id/correlation_id.
   `duplicate` означает повтор события, а не создание второго уведомления.
7. Проверьте inbox и notifications. `created` означает сохранение в БД; внешней доставки пока нет.
8. Откройте trace_id в Jaeger. Для асинхронного этапа используйте отдельную consumer trace
   и её link на исходный trace context.

Примеры LogQL в Grafana Explore, источник Loki:

```logql
{namespace="sharetrip"} | json | correlation_id="publication-456"
{namespace="sharetrip", app="notification"} | json | event_id="78910abc-1234-4567-89ab-1234567890ab"
```

SQL для БД соответствующего сервиса, только чтение:

```sql
-- ShareTrip
SELECT id, status, company_id FROM trips WHERE id = '<trip_uuid>';
SELECT id, status, attempts, last_error, sent_at,
       payload->>'correlation_id' AS correlation_id,
       payload->>'causation_id' AS causation_id
FROM outbox_event WHERE aggregate_id = '<trip_uuid>';

-- Notification
SELECT event_id, processed_at FROM processed_events WHERE event_id = '<event_uuid>';
SELECT id, status, created_at FROM notifications WHERE payload->>'trip_id' = '<trip_uuid>';
```

Для локальных логов pod-ов:

```cmd
kubectl --context docker-desktop logs -n sharetrip deployment/sharetrip --since=30m
kubectl --context docker-desktop logs -n sharetrip deployment/contract --since=30m
kubectl --context docker-desktop logs -n sharetrip deployment/notification --since=30m
```

Полные тела запросов и событий, токены и DSN намеренно не записываются.
Валидация пишет только имена полей; HTTP-клиент не копирует тексты ошибок Contract
и принимает только известные коды причин отказа. Остальные технические ошибки
по-прежнему требуют осторожности: это не универсальный механизм удаления секретов. Используйте идентификаторы, а не
персональные данные как ключи поиска. Адреса стенда: [addresses.md](addresses.md).

## Диаграмма

[PlantUML sequence diagram](diagrams/trip-publication-observability.puml).


## Правила логирования и измерений

`service` — приложение (`sharetrip`, `contract`, `notification`), `component` — компонент,
`operation` — операция, `result` — её итог. Эти же имена приложений
используются в OpenTelemetry `service.name`; старые трассы ShareTrip сохраняют имя `share-trip`. Инициатор в логах и trace attributes называется
`client_id`; диагностический HTTP header `X-User-ID` сохранён для совместимости.

API, HTTP-клиент и Kafka consumer пишут итог своей операции: успех и бизнес-отказ — Info,
некорректный вход — Warn, техническая ошибка — Error. Нижние слои возвращают ошибки
с контекстом; промежуточные шаги, начало/commit транзакций и итоги сервисных методов — Debug.
Ошибка rollback пишется отдельно, поскольку это самостоятельная ошибка очистки.

HTTP-метрики используют label `route` с шаблоном маршрута, без конкретных UUID.
Backlog измеряется отдельной транзакцией с таймаутом 1 секунда после фиксации batch.
При ошибке подсчёта Gauge сохраняет последнее значение; результат отправки не откатывается.
`sharetrip_outbox_pending_total` переименована в `sharetrip_outbox_pending`,
`notification_send_*` — в `notification_persist_*`; dashboard использует новые имена.
Исторические ряды со старыми именами остаются в Prometheus до истечения retention.
