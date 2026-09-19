SHELL := /bin/bash

.PHONY: dev down logs test test-integration lint migrate rollback seed search-quality benchmark backup restore-test health

dev:
	docker compose up --build -d
	$(MAKE) migrate

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...

migrate:
	docker compose run --rm backend migrate

rollback:
	@echo "Rollback is migration-specific; destructive automatic down migrations are intentionally disabled."

seed:
	@echo "TODO Sprint 02: seed canonical data"

search-quality:
	@echo "TODO Sprint 08: quality evaluation"

benchmark:
	@echo "TODO Sprint 10/21: benchmark suite"

backup:
	@echo "TODO Sprint 16: backup runner"

restore-test:
	@echo "TODO Sprint 16: restore test"

health:
	curl -fsS http://localhost/health/live && echo
	curl -fsS http://localhost/health/ready && echo
