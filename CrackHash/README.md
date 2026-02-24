POST /api/hash/crack — создать задачу.

DELETE /api/hash/crack — отменить задачу.

GET /api/hash/status — проверить статус задачи.

GET /api/metrics — проверить метрики.

curl -X POST http://localhost:57107/api/hash/crack \
  -H "Content-Type: application/json" \
  -d '{
        "hash": "5d41402abc4b2a76b9719d911017c592",
        "maxLength": 5,
        "algorithm": "md5",
        "alphabet": "abcdefghijklmnopqrstuvwxyz"
      }'

curl "http://localhost:57107/api/hash/status?requestId=99100f0b-5dd7-478c-82e4-0ed0db74fbd4"

curl -X DELETE "http://localhost:57107/api/hash/crack?requestId=99100f0b-5dd7-478c-82e4-0ed0db74fbd4"

curl http://localhost:57107/api/metrics