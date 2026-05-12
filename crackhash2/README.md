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

## Автотесты отказоустойчивости

Скрипт покрывает 6 сценариев:
- стоп сервиса `manager`;
- стоп `dispatcher`;
- стоп primary-ноды MongoDB replica set;
- стоп `rabbitmq`;
- стоп одного `worker` во время обработки;
- отсутствие `worker` в момент создания задания.

Запуск:

```bash
./scripts/failure-tests.sh
```

Скрипт сам:
- поднимает инфраструктуру (`docker compose up -d --build --scale worker=2`);
- прогоняет все кейсы последовательно;
- делает проверки через HTTP API менеджера;
- в конце печатает `PASS/FAIL` и очищает окружение (`docker compose down -v`).