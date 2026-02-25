## Описание

Сервис CrackHash реализует распределённый перебор хэшей.  
Есть два компонента:

- **Manager** — принимает внешние HTTP‑запросы, создаёт задачи, разбивает их на части и распределяет по воркерам, собирает результаты.
- **Worker** — получает часть задачи от менеджера по HTTP, перебирает свой диапазон и отправляет результат обратно менеджеру.

Взаимодействие между `manager` и `worker` происходит **по HTTP внутри сети docker-compose**, домены соответствуют именам сервисов: `http://manager:57107` и `http://worker:57107`.

---

## Требования

- Docker и docker-compose  
- Либо Go ≥ 1.23 (если хотите запускать бинарники локально без Docker)

---

## Сборка и запуск через Docker Compose (рекомендуется)

1. Собрать образы:

```bash
docker-compose build
```

2. Запустить менеджер и одного воркера:

```bash
docker-compose up
```

По умолчанию:

- менеджер слушает `http://localhost:57107`
- внутри docker‑сети воркер доступен как `http://worker:57107`

Переменные окружения по умолчанию см. в `docker-compose.yml`.

---

## Масштабирование количества воркеров

Количество воркеров задаётся:

- через переменную окружения **`WORKERS_COUNT`** у сервиса `manager`
- и опцией `--scale worker=N` у docker-compose

Менеджер внутри `cmd/manager/main.go` использует:

- список `WORKER_URLS` (если задан явно),
- иначе `WORKERS_COUNT` (одинаковые URL `http://worker:57107`),
- иначе значение по умолчанию `http://worker:57107`.

### Примеры

**1 воркер:**

```bash
WORKERS_COUNT=1 docker-compose up --scale worker=1
```

**4 воркера:**

```bash
WORKERS_COUNT=4 docker-compose up --scale worker=4
```

**8 воркеров:**

```bash
WORKERS_COUNT=8 docker-compose up --scale worker=8
```

Во всех случаях менеджер общается с воркерами по HTTP‑адресу `http://worker:57107/...` (имя сервиса `worker` внутри сети docker-compose).

---

## Локальный запуск без Docker (опционально)

1. Собрать бинарники:

```bash
go build -o manager ./cmd/manager
go build -o worker ./cmd/worker
```

2. Запустить менеджер:

```bash
export WORKERS_COUNT=1          # или 4, 8
./manager
```

3. В другом терминале запустить воркер:

```bash
export MANAGER_URL=http://localhost:57107
export PORT=57107
./worker
```

В этом режиме менеджер и воркер общаются по `http://localhost:57107`, а не по docker‑доменам.

---

## HTTP API менеджера

- **POST** `/api/hash/crack` — создать задачу перебора.
- **DELETE** `/api/hash/crack` — отменить задачу.
- **GET** `/api/hash/status` — проверить статус задачи.
- **GET** `/api/metrics` — получить метрики.

### Формат запросов

**Создание задачи**

```bash
curl -X POST http://localhost:57107/api/hash/crack \
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
  "estimatedCombinations": 456976
}
```

**Проверка статуса**

```bash
curl "http://localhost:57107/api/hash/status?requestId=<REQUEST_ID>"
```

**Отмена задачи**

```bash
curl -X DELETE "http://localhost:57107/api/hash/crack?requestId=<REQUEST_ID>"
```

**Метрики**

```bash
curl http://localhost:57107/api/metrics
```

---

## Тест-кейсы

Ниже несколько примеров запросов, которые можно использовать для проверки корректности работы системы (менеджер + один или несколько воркеров).

### Тест‑кейс 1: простой успешный перебор (слово `test`)

- Слово: `test`
- Алгоритм: `MD5`
- Хэш: `098f6bcd4621d373cade4e832627b4f6`
- Алфавит: только маленькие латинские буквы
- Максимальная длина: 4

**Запрос:**

```bash
curl -X POST http://localhost:57107/api/hash/crack \
  -H "Content-Type: application/json" \
  -d '{
        "hash": "098f6bcd4621d373cade4e832627b4f6",
        "maxLength": 4,
        "algorithm": "MD5",
        "alphabet": "abcdefghijklmnopqrstuvwxyz"
      }'
```

Сохраните `requestId` из ответа и через несколько секунд проверьте статус:

```bash
curl "http://localhost:57107/api/hash/status?requestId=<REQUEST_ID>"
```

Ожидаемо:

- `status` перейдёт в `READY`,
- в `data` будет содержаться строка `"test"` (или включать её).

---

### Тест‑кейс 2: другой хэш длины 4 (слово `aaaa`)

- Слово: `aaaa`
- Алгоритм: `MD5`
- Хэш: `74b87337454200d4d33f80c4663dc5e5`
- Алфавит: маленькие латинские буквы
- Максимальная длина: 4

```bash
curl -X POST http://localhost:57107/api/hash/crack \
  -H "Content-Type: application/json" \
  -d '{
        "hash": "74b87337454200d4d33f80c4663dc5e5",
        "maxLength": 4,
        "algorithm": "MD5",
        "alphabet": "abcdefghijklmnopqrstuvwxyz"
      }'
```

Далее аналогично проверить статус:

```bash
curl "http://localhost:57107/api/hash/status?requestId=<REQUEST_ID>"
```

Ожидаемо в `data` будет найдено слово `"aaaa"`.

---

### Тест‑кейс 3: хэш вне пространства поиска

- Слово: `hello` (длина 5)
- Алгоритм: `MD5`
- Хэш: `5d41402abc4b2a76b9719d911017c592`
- Алфавит: маленькие латинские буквы
- Максимальная длина: 4

```bash
curl -X POST http://localhost:57107/api/hash/crack \
  -H "Content-Type: application/json" \
  -d '{
        "hash": "5d41402abc4b2a76b9719d911017c592",
        "maxLength": 4,
        "algorithm": "MD5",
        "alphabet": "abcdefghijklmnopqrstuvwxyz"
      }'
```

Через какое‑то время:

```bash
curl "http://localhost:57107/api/hash/status?requestId=<REQUEST_ID>"
```

Ожидаемо:

- `status` будет `READY`,
- `data` либо пустой, либо не содержит `"hello"`, т.к. длина слова 5, а поиск ограничен длиной 4.

---

### Тест‑кейс 4: проверка масштабируемости (4 воркера)

1. Запустить систему с четырьмя воркерами:

```bash
WORKERS_COUNT=4 docker-compose up --scale worker=4
```

2. Отправить сразу несколько задач (например, тест‑кейс 1 и 2 по несколько раз подряд).

3. Убедиться по логам и по времени ответа, что вычисления распределяются между воркерами и система остаётся отзывчивой.  
   Дополнительно можно смотреть `GET /api/metrics`, чтобы оценить количество активных и завершённых задач.