.PHONY: help up up-infra down down-v build restart logs logs-app logs-worker ps \
        run-web run-worker

.DEFAULT_GOAL := help

GREEN  := \033[0;32m
YELLOW := \033[0;33m
RESET  := \033[0m

## ─── HELP ────────────────────────────────────────────────────

help: ## Tampilkan daftar perintah
	@echo "TG-Manager — available commands:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-14s$(RESET) %s\n", $$1, $$2}'



## ─── DOCKER ──────────────────────────────────────────────────

up: ## Jalankan semua service
	@echo "$(GREEN)Starting all services...$(RESET)"
	docker compose up -d
	@echo "$(GREEN)Semua service berjalan!$(RESET)"
	@echo "  App      → http://localhost:8080"
	@echo "  RabbitMQ → http://localhost:15672"
	@echo "  MinIO    → http://localhost:9001"
	@echo "  Postgres → localhost:5433"
	@echo "  Redis    → localhost:6380"

up-infra: ## Jalankan hanya infrastruktur (tanpa app & worker)
	@echo "$(GREEN)Starting infrastructure only...$(RESET)"
	docker compose up -d postgres pgadmin redis rabbitmq minio minio-init

down: ## Stop semua service
	docker compose down

down-v: ## Stop & hapus semua data (HATI-HATI!)
	@echo "$(YELLOW)WARNING: Semua data akan dihapus!$(RESET)"
	@read -p "Lanjut? (y/N) " confirm && [ "$$confirm" = "y" ]
	docker compose down -v

build: ## Build ulang Docker image
	docker compose build --no-cache app worker

restart: ## Restart app & worker tanpa rebuild
	docker compose restart app worker

logs: ## Lihat log semua service
	docker compose logs -f

logs-app: ## Lihat log app saja
	docker compose logs -f app

logs-worker: ## Lihat log worker saja
	docker compose logs -f worker

ps: ## Status semua container
	docker compose ps


run-web: ## Jalankan web server langsung (tanpa Docker)
	@echo "$(GREEN)Starting web server...$(RESET)"
	go run cmd/web/main.go

run-worker: ## Jalankan worker langsung (tanpa Docker)
	@echo "$(GREEN)Starting worker...$(RESET)"
	go run cmd/worker/main.go