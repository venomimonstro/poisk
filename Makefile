SHELL := /bin/bash

.PHONY: dev down logs test test-integration lint frontend-check migrate rollback seed search-quality benchmark capacity-status candidate readiness release-gate fresh-install-gate upgrade-gate backup backup-verify recovery-gate restore-test health

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
	@test -n "$$TEST_DATABASE_URL" || (echo "TEST_DATABASE_URL is required" >&2; exit 2)
	go test -tags=integration ./...

lint:
	go vet ./...

frontend-check:
	cd web && npm ci && npm run lint && npm run build

migrate:
	docker compose run --rm backend migrate

rollback:
	@echo "Database down-migrations are intentionally disabled. Use the immutable release registry + verified recovery procedure."

seed:
	@echo "Canonical data is populated through crawler/import pipelines; there is no generic production seed target."

search-quality:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	docker compose run --rm backend quality

benchmark:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	docker compose run --rm backend capacityctl benchmark "$${CAPACITY_LABEL:-manual-capacity}"

capacity-status:
	docker compose run --rm backend capacityctl status 10

candidate:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	docker compose run --rm backend readinessctl candidate

readiness:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	docker compose run --rm backend readinessctl check

release-gate:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	bash scripts/release_candidate_gate.sh

fresh-install-gate:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	bash scripts/fresh_install_gate.sh

upgrade-gate:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	bash scripts/upgrade_gate.sh

backup:
	@test "$$BACKUP_QUIESCED" = "yes" || (echo "BACKUP_QUIESCED=yes is required after API/mail mutators and workers are stopped" >&2; exit 2)
	docker compose --profile ops run --rm backup

backup-verify:
	@test -n "$$BACKUP_DIR" || (echo "BACKUP_DIR is required and must be the container path /backups/<timestamp>" >&2; exit 2)
	docker compose --profile ops run --rm --entrypoint sh backup /ops/verify.sh "$$BACKUP_DIR"

recovery-gate:
	@test -n "$$READINESS_GIT_SHA" || (echo "READINESS_GIT_SHA is required" >&2; exit 2)
	@test -n "$$RELEASE_VERSION" || (echo "RELEASE_VERSION is required" >&2; exit 2)
	bash scripts/recovery_gate.sh

restore-test: recovery-gate

health:
	curl -fsS http://localhost/health/live && echo
	curl -fsS http://localhost/health/ready && echo
