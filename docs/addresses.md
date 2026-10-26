# Адреса локального стенда

Запуск из репозитория share-trip: `make k8s-up`. Адреса ниже доступны с Windows,
в том числе из Postman. Дополнительный port-forward для них не нужен.
Источник портов: [k8s/infra/access.yaml](../k8s/infra/access.yaml).

## Сервисы и интерфейсы

| Компонент | Адрес |
| --- | --- |
| ShareTrip | http://localhost:38080 |
| Contract | http://localhost:38082 |
| Notification | http://localhost:38081 |
| Keycloak | http://localhost:38087 |
| Grafana | http://localhost:33000 |
| Prometheus | http://localhost:39090 |
| Kafka UI | http://localhost:38085 |
| Jaeger | http://localhost:36686 |

## API для Postman

Заменяйте `{trip_id}` и `{notification_id}` на UUID соответствующей записи.

| Действие | Метод | URL |
| --- | --- | --- |
| Создать поездку | POST | `http://localhost:38080/api/trip/create` |
| Опубликовать поездку | POST | `http://localhost:38080/api/trip/publish` |
| Получить поездку | GET | `http://localhost:38080/api/trip/{trip_id}` |
| Метрики ShareTrip | GET | `http://localhost:38080/metrics` |
| Создать договор | POST | `http://localhost:38082/api/contract/create` |
| Активировать договор | POST | `http://localhost:38082/api/contract/activate` |
| Получить договор | POST | `http://localhost:38082/api/contract/get-by-id` |
| Получить активный договор | POST | `http://localhost:38082/api/contract/get-active` |
| Проверить услугу | POST | `http://localhost:38082/api/option/check` |
| Метрики Contract | GET | `http://localhost:38082/metrics` |
| Метрики Notification | GET | `http://localhost:38081/metrics` |
| OpenAPI договора | GET | `http://localhost:38082/api/contract.yaml` |
| Создать уведомление | POST | `http://localhost:38081/api/notification/` |
| Получить уведомление | GET | `http://localhost:38081/api/notification/{notification_id}` |
| Получить или обновить токены | POST | `http://localhost:38087/realms/sharetrip/protocol/openid-connect/token` |

Для JSON-запросов задайте `Content-Type: application/json`.
Для всех трёх операций поездок нужен заголовок `X-Refresh-Token: <refresh_token>`.
`GET /metrics` доступен без токена во всех трёх сервисах. Прикладные ответы используют оболочку `data` / `errors`;
ошибки middleware могут иметь другой формат.

Получение токенов: [deploy/keycloak/auth.http](../deploy/keycloak/auth.http), первый запрос.
В Postman выберите Body → x-www-form-urlencoded и передайте `client_id=sharetrip-api`,
`client_secret`, `username`, `password`, `grant_type=password`. Используйте свои значения
из Keycloak. Для обновления передайте `client_id`, `client_secret`, `grant_type=refresh_token`
и `refresh_token`. Это не JSON-запрос.

После переноса с Compose получите новые токены: внешний issuer теперь
`http://localhost:38087/realms/sharetrip`. В токене нужны роль `client` клиента
`sharetrip-api` и атрибут `company_id`.

Пример создания поездки, Body → raw → JSON:

```json
{
  "from_point": "Москва",
  "to_point": "Казань",
  "departure_time": "2027-11-15T10:00:00+03:00",
  "available_seats": 3
}
```

Время отправления должно быть в будущем. `company_id` берётся из токена;
для создания поездки у компании должна быть доступна услуга `trip_creation`, для публикации — `trip_publication`.

Пример публикации:

```json
{
  "trip_id": "f9b711e3-8f45-4dc3-8cab-07623f5de6e5"
}
```

Подставьте ID из ответа создания поездки. Создание возвращает HTTP 200;
первая публикация — 201, повторная — 200.

## Адреса внутри Kubernetes

Эти имена используются pod-ами namespace `sharetrip`, а не Postman на Windows.

| Компонент | Внутренний адрес |
| --- | --- |
| ShareTrip | `http://sharetrip-service:8080` |
| Contract | `http://contract-service:8080` |
| Notification | `http://notification-service:8080` |
| Keycloak | `http://keycloak:8080` |
| PostgreSQL ShareTrip | `postgres-sharetrip:5432` |
| PostgreSQL Contract | `postgres-contract:5432` |
| PostgreSQL Notification | `postgres-notification:5432` |
| Kafka, bootstrap / controller | `kafka:9092` / `kafka:9093` |
| Kafka UI | `http://kafka-ui:8080` |
| Grafana | `http://grafana:3000` |
| Prometheus | `http://prometheus:9090` |
| Loki | `http://loki:3100` |
| OTel Collector, HTTP / gRPC | `otel-collector:4318` / `otel-collector:4317` |
| Jaeger, UI / OTLP gRPC | `http://jaeger:16686` / `jaeger:4317` |
| PostgreSQL Exporter, по одному на БД | `postgres-exporter-sharetrip:9187`, `postgres-exporter-contract:9187`, `postgres-exporter-notification:9187` |

Kafka Exporter доступен внутри кластера по адресу `http://kafka-exporter:9308/metrics`;
Dashboard процесса: [Trip publication](http://localhost:33000/d/trip-publication).

Alloy читает логи pod-ов через Kubernetes API; отдельного Service у него нет.
PostgreSQL, Kafka-брокер, Loki, Collector и Exporter наружу не опубликованы.
Старые порты Compose `6543/6544/6545`, `9092`, `8087`, `8085` и порты приложений
из IDE `4001/4002` не относятся к текущему запуску Kubernetes.

Инструкции запуска, настройки и диагностика: [k8s/README.md](../k8s/README.md).
