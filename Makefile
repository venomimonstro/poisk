SHELL := /bin/bash

.PHONY: dev down logs test test-integration lint frontend-check migrate rollback seed search-quality benchmark capacity-status candidate readiness release-gate backup restore-test health

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

backup:
	@echo "A backup is not evidence until a real backup artifact is produced and recorded with exact release/commit/schema metadata."
	@echo "Run the environment-specific backup procedure, then use: ./app recoveryctl record BACKUP ..."
	@echo "See docs/runbooks/security-commercial-readiness.md"
	@exit 2

restore-test:
	@echo "A restore drill must restore a real artifact into an isolated database and validate it before recording PASS."
	@echo "After the drill use: ./app recoveryctl record RESTORE ..."
	@echo "See docs/runbooks/security-commercial-readiness.md"
	@exit 2

health:
	curl -fsS http://localhost/health/live && echo
	curl -fsS http://localhost/health/ready && echo
