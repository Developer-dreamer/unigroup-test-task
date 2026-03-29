# Product Microservice with Outbox Pattern

Це тестове завдання на реалізацію мікросервісу управління товарами. Замість SQS було використано Redis Streams. Для гарантованої доставки повідомлень реалізовано патерн Outbox. 

## 🚀 Швидкий запуск

API, Notification та вся необхідна інфраструктура розгортаються однією командою.

Перед запуском переконайтеся, що на вашій машині вільні наступні порти:
- `8080` (API)
- `5432` (PostgreSQL)
- `6379` (Redis)
- `9090` (Prometheus)

Також створіть файл .api_env
```dotenv
YAML_CFG_DIR=/app/config/app/api.yaml
POSTGRES_PASSWORD=password
```
та .notif_env
```dotenv
YAML_CFG_DIR=/app/config/app/notif.yaml
```

Тепер запустіть цю команду в root директорії проєкту:
```bash
docker-compose -f deployment/docker/docker-compose.yml up --build -d
````

## 🛠 Архітектурні рішення

- **Redis Streams:** Використовується як message broker між API та Worker-сервісами.
- **Transactional Outbox:** Дані товару та події для брокера зберігаються в базі даних в межах однієї транзакції. Окремий фоновий Relay-процес вичитує ці події та відправляє їх в Redis, гарантуючи, що жодна подія не загубиться навіть при падінні брокера чи мережі.
- **Graceful Shutdown:** Сервіс коректно завершує всі фонові процеси, закриває з'єднання з БД та зупиняє HTTP-сервер.

## 📡 API Examples

Нижче наведено приклади запитів для тестування API (скопіюйте в термінал):

### 1\. Створення товару (POST)

```bash
curl -X POST http://localhost:8080/products \
-H "Content-Type: application/json" \
-d '{
  "name": "Test Product",
  "description": "This is a test product",
  "seller": "123e4567-e89b-12d3-a456-426614174000",
  "price": 1500,
  "amount" : 1
}'
```

### 2\. Отримання списку товарів (GET)

```bash
curl -X GET "http://localhost:8080/products?limit=5&offset=0"
```

### 3\. Видалення товару (DELETE)

*Замініть `<UUID>` на реальний ID товару, отриманий з попередніх запитів.*

```bash
curl -X DELETE "http://localhost:8080/products/<UUID>"
```

## 📊 Метрики (Prometheus)

Сервіс експонує метрики про кількість створених та видалених товарів.

1.  Відкрийте дашборд Prometheus у браузері: [http://localhost:9090](https://www.google.com/search?q=http://localhost:9090)
2.  У рядку пошуку введіть одну з наступних метрик та натисніть **Execute**:
    - `products_created_total`
    - `products_deleted_total`