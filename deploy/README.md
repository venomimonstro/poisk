# Runtime network model

## Public surface

Только `nginx:80` публикуется на host в MVP.

```text
Internet
  ↓
Nginx :80
  ├─ / → frontend:3000
  ├─ /api/* → backend:8080
  └─ /health/* → backend:8080
```

## Internal-only services

Следующие порты не публикуются на host:

- PostgreSQL/PostGIS `5432`
- Manticore SQL `9306`
- Manticore HTTP `9308`
- backend `8080`
- frontend `3000`

Доступ между сервисами идёт через внутреннюю Docker-сеть.

## Security principles

- не добавлять public port mapping для PostgreSQL/Manticore;
- секреты передавать только через `.env`/production secret mechanism;
- `.env` не коммитить;
- `no-new-privileges` использовать для application containers;
- production TLS/CDN/WAF подключается перед public Gate A.
