# ShareTrip

Адреса сервисов и запросы Postman: [docs/addresses.md](docs/addresses.md).

Конфигурация, обязательные переменные, Kubernetes и обновление секретов:
[k8s/README.md](k8s/README.md).

Бизнес-описание проекта: [docs/README.md](docs/README.md).

## Как расследовать: уведомление не пришло

Найдите PublishTrip по request_id или trip_id, затем по correlation_id проследите
проверку Contract, запись outbox, Kafka и обработку inbox в Notification.
HTTP 201 означает публикацию поездки и сохранение outbox; доставку уведомления он не гарантирует.
Сейчас Notification сохраняет уведомление в БД; внешний SMS/email-провайдер не подключён.

[Dashboard Trip publication](http://localhost:33000/d/trip-publication).
Пошаговая инструкция, метрики, запросы Loki/SQL и диаграмма:
[docs/observability.md](docs/observability.md).
