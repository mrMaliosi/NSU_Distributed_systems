## HTTP API менеджера

- **POST** `/api/hash/crack` — создать задачу перебора.
- **DELETE** `/api/hash/crack` — отменить задачу.
- **GET** `/api/hash/status` — проверить статус задачи.
- **GET** `/api/metrics` — получить метрики.

### Формат запросов

### Запуск инфраструктуры

```bash
docker compose up -d
```

MongoDB запускается как replica set `rs0` (1 primary + 2 secondary), RabbitMQ — с persistent-хранилищем.

**Создание задачи**

```bash
curl -X POST http://localhost:8081/api/hash/crack \
  -H "Content-Type: application/json" \
  -d '{
        "hash": "098f6bcd4621d373cade4e832627b4f6",
        "maxLength": 4,
        "algorithm": "MD5",
        "alphabet": "abcdefghijklmnopqrstuvwxyz"
      }'
```

Ответ:

```json
{
  "requestId": "UUID-задачи",
  "estimatedCombinations": 475254
}
```

**Проверка статуса**

```bash
curl "http://localhost:8081/api/hash/status?requestId=<REQUEST_ID>"
```

**Отмена задачи**

```bash
curl -X DELETE "http://localhost:8081/api/hash/crack?requestId=<REQUEST_ID>"
```

**Метрики**

```bash
curl http://localhost:8081/api/metrics
```