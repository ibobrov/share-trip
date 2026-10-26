# Producer события TripPublished

ShareTrip сохраняет событие вместе с публикацией поездки; фоновый publisher отправляет его в Kafka. Consumer реализован
в отдельном проекте `share-trip-notification`: он создаёт уведомление и защищается
от повторной обработки через `processed_events`.

## Последовательность

Ниже указаны настройки по умолчанию. Таймауты, размеры пачки и число попыток
задаются через env; список параметров — в [k8s/README.md](../k8s/README.md).

1. Отдельным SELECT без блокировки и явной транзакции читаются данные поездки.
2. Проверяется владелец и разрешение `trip_publication` через Contract Service.
3. В транзакции поездка повторно читается через SELECT FOR UPDATE, проверяются
   владелец и актуальный статус, сохраняются статус, история и запись outbox.
4. После COMMIT API возвращает 201, не ожидая Kafka. Повторная публикация возвращает 200 без нового события.
5. Фоновый publisher каждую секунду читает до 10 pending-событий под `FOR UPDATE SKIP LOCKED` в отдельной транзакции.
6. Producer отправляет событие и ждёт ACK. Publisher отмечает `sent`, увеличивает `attempts`, очищает `last_error`, заполняет `sent_at` и делает COMMIT.
7. При ошибке отправки сохраняются `attempts` и `last_error`: до десяти ошибок статус остаётся `pending`, с десятой — `failed`. Остальные события пачки продолжают обрабатываться. При ошибке БД вся пачка откатывается.

Диаграммы: [потеря без outbox](diagrams/lost-event-without-outbox.puml),
[transactional outbox](diagrams/publish-trip-events.puml),
[inbox transaction](diagrams/inbox-transaction.puml),
[полный процесс](diagrams/trip-notification-outbox-inbox.puml).

## Контракт события

Topic: `trip.events`. Ключ сообщения: строковый UUID поездки `trip_id`.
Hash-балансировщик направляет события с одинаковым ключом в одну partition
при неизменном количестве partitions.

```json
{
  "event_id": "78910abc-1234-4567-89ab-1234567890ab",
  "event_type": "TripPublished",
  "trip_id": "f9b711e3-8f45-4dc3-8cab-07623f5de6e5",
  "client_id": "a1b2c3d4-1111-4111-8111-123456789abc",
  "company_id": "company-acme",
  "occurred_at": "2026-10-22T20:00:00Z",
  "request_id": "request-123",
  "correlation_id": "publication-456",
  "causation_id": "request-123",
  "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
}
```

Идентификатор события совпадает с `outbox_event.id`. Компания и клиент берутся
из записи, прочитанной под блокировкой. Время события в UTC соответствует времени
перехода статуса и записи истории. В outbox сохраняются trip_id, client_id и company_id;
request_id, correlation_id, causation_id и исходный trace context также сохраняются в JSON payload;
время и идентификатор события находятся в отдельных столбцах. Внутреннее имя события outbox
остаётся `trip_published`, внешнее значение `event_type` — `TripPublished`.

Metadata передаётся также в Kafka headers. Consumer создаёт новую trace со span link
на контекст отправителя. Метрики, dashboard и правила поиска связанного процесса:
[observability.md](observability.md).

## Настройки и запуск

Текущий полный стенд запускается из share-trip:

```text
make k8s-up
```

В Kubernetes приложения используют `KAFKA_BROKERS=kafka:9092` и `KAFKA_TRIP_TOPIC=trip.events`.
Данные Kafka хранятся в PVC `kafka`, каталог `/var/lib/kafka/data`.
Job `kafka-init` идемпотентно создаёт topic `trip.events`: три partitions,
replication factor 1. При другом `KAFKA_TRIP_TOPIC` нужно отдельно создать соответствующий topic
и согласовать настройки producer и consumer.

Kafka UI: http://localhost:38085. После успешной новой публикации сообщение
доступно в разделе Messages topic `trip.events`. Consumer group `notification-service`
появляется после запуска Notification Service; после обработки продвигается её offset.
Сам брокер наружу не опубликован. `kafka:9092` недоступен напрямую с Windows.

Для прежнего запуска через Compose остаются listener `localhost:9092` для Windows,
`kafka:29092` внутри сети Compose и `host.docker.internal:19092` для pod-ов, обращающихся
к Compose. Эти адреса не используются текущим стендом Kubernetes.
Карта адресов: [addresses.md](addresses.md).

Producer использует `acks=all`, отправку без ожидания набора batch и до трёх
попыток kafka-go в пределах контекста отправки. Отправка ограничена одной секундой
и более коротким дедлайном вызывающего кода. Пачка ограничена 15 секундами.
HTTP-публикация имеет лимит 2 секунды, как создание и получение: до 1,2 секунды на Contract Service, остальное на БД.

При штатном завершении worker отменяется и завершается до закрытия producer и пула БД.
HTTP-обработчикам даётся до 5 секунд на завершение.

## Таблицы и повторы

Используется существующая `outbox_event`. Миграция `20261023160000_add_outbox_delivery.sql`
добавляет поля доставки и частичный индекс для pending, не создавая новую таблицу.
`attempts` считает вызовы отправки worker, включая успешный; внутренние повторы kafka-go отдельно не учитываются.

Если Kafka приняла событие, а отметка `sent` не сохранилась, событие будет отправлено повторно
с тем же `event_id`. Notification Service использует существующую `processed_events`:
создание уведомления и запись event_id атомарны, offset подтверждается после COMMIT.
Это доставка с возможными дублями, а не exactly-once между PostgreSQL и Kafka.

События `failed` сохраняются для разбора и не выбираются автоматически.
После устранения причины их можно вручную вернуть в `pending` со сбросом `attempts`.
Записи старого формата с `driver_id` требуют исправления payload на `client_id`;
worker не отправляет событие с отсутствующим получателем. Существующие строки при миграции
получают `pending` и будут отправлены: для ранее доставленных событий защитой остаётся inbox.

## Тесты

- API-тесты проверяют запись pending-события и ноль попыток отправки после HTTP-запроса.
- Service-тесты проверяют откат при ошибке домена, outbox или COMMIT.
- `internal/api_test/outbox_publisher_test.go` проверяет повтор после сбоя, сохранение event_id,
  лимит попыток, параллельных worker и повтор после ACK без успешной записи sent.
  Каждый тест использует отдельную схему в тестовом PostgreSQL.
- `internal/api_test/outbox_kafka_test.go` проверяет сохранение события при недоступном брокере
  и последующую доставку из PostgreSQL в настоящую Kafka через временный topic.
- Unit-тесты адаптера проверяют JSON, ключ и ошибки; отдельный Kafka-тест проверяет сам producer.

Пример для отдельного тестового брокера, доступного с Windows на `localhost:9092`.
Текущий брокер Kubernetes по этому адресу не опубликован: простой port-forward
не меняет advertised-адрес `kafka:9092`. Не используйте его как адрес Windows-клиента.
PowerShell:

```powershell
$env:KAFKA_TEST_BROKERS = 'localhost:9092'
go test ./internal/api_test -run TestOutboxPublisher -count=1 -v
go test ./internal/events -run TestProducer_Kafka -count=1 -v
```

Без `KAFKA_TEST_BROKERS` реальный Kafka-тест пропускается. Остальные тесты адаптера
и unit-тесты сервиса работают без Kafka; интеграционным API-тестам нужен Docker
для PostgreSQL.
