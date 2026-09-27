# Runtime network model

## Public surface

Только `nginx:80` публикуется на host в Compose MVP.

```text
Internet
  ↓
TLS/CDN/WAF or trusted reverse proxy
  ↓
Nginx :80
  ├─ / → frontend:3000
  ├─ /api/* → backend:8080
  └─ /health/* → backend:8080
```

`docker-compose.yml` **не является TLS-терминатором**. Коммерческий production обязан иметь внешний HTTPS edge с валидным сертификатом и HTTP→HTTPS redirect до Nginx. `readinessctl check` не может стать READY без `EDGE_TLS_PROXY` evidence для текущего commit/schema.

## Trusted real client IP

Nginx намеренно перезаписывает `X-Forwarded-For` значением `$remote_addr` и не доверяет произвольному входящему XFF. Это безопасный default для прямого подключения.

Если перед Nginx стоит CDN/LB, real-client-IP разрешается только для **явно перечисленных CIDR доверенного провайдера**. Нельзя включать `real_ip_header X-Forwarded-For` без `set_real_ip_from` allowlist. После настройки нужно подтвердить:

- spoofed XFF от недоверенного клиента не меняет effective IP;
- два клиента через CDN получают разные effective IP;
- rate limiting не объединяет весь CDN в одного клиента;
- scheme/host остаются корректными после TLS termination.

Результат фиксируется как `EDGE_TLS_PROXY` evidence.

## Internal-only services

Следующие порты не публикуются на host:

- PostgreSQL/PostGIS `5432`
- Manticore SQL `9306`
- Manticore HTTP `9308`
- backend `8080`
- frontend `3000`
- `/internal/mail-gateway/*` не проксируется публичным Nginx и предназначен только для MTA bridge во внутренней сети.

Доступ между сервисами идёт через внутреннюю Docker-сеть.

## Backup / restore with Mail blobs

PostgreSQL и `mail_blob_data` — единый canonical dataset. Backup format `poisk-v2` включает `postgres.dump` и checksum-protected `mail-blobs.tar.gz`. Manticore не входит в backup, потому что восстанавливается из canonical PostgreSQL.

### Backup

1. Открыть maintenance window.
2. Остановить API и все процессы, которые могут создавать/удалять письма или вложения, включая Internet Mail workers. PostgreSQL должен остаться запущен.
3. Только после этого выполнить backup с явным подтверждением quiesced state:

```bash
BACKUP_QUIESCED=yes docker compose --profile ops run --rm backup
```

Скрипт откажется работать без `BACKUP_QUIESCED=yes`. Этот флаг является подтверждением оператора, а не механизмом остановки writers.

### Restore drill

Restore также выполняется только в maintenance window. Перед запуском необходимо остановить API/mail writers и проверить выбранный backup directory.

```bash
RESTORE_DIR=/backups/<timestamp> \
RESTORE_CONFIRM='RESTORE:poisk' \
RESTORE_QUIESCED=yes \
RESTORE_ALLOW_NONEMPTY=yes \
docker compose --profile ops run --rm restore
```

Restore до изменения БД проверяет SHA-256, формат backup, безопасные UUID blob names и точное совпадение списка файлов с tar archive. После восстановления БД каждый `mail_attachment_blobs.storage_key` проверяется на наличие физического `<uuid>.blob`. Успешный реальный drill фиксируется через `recoveryctl` как `RESTORE PASS` на текущей schema.

## Security principles

- не добавлять public port mapping для PostgreSQL/Manticore/backend/frontend;
- секреты передавать только через `.env`/production secret mechanism;
- `.env` не коммитить;
- application containers запускать non-root, где это поддерживается, и с `no-new-privileges`;
- не доверять proxy headers без CIDR allowlist;
- production TLS/CDN/WAF является обязательным commercial gate;
- `READY` разрешён только по фактическим evidence, а не по наличию кода или документации.
