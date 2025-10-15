# =============================================
# Makefile for Baseline Project DevOps Control
# =============================================
# Paths
APP_COMPOSE = ./baseline-project/docker-compose.yml
OBS_COMPOSE = ./observability/docker-compose.yml

# Default target
.DEFAULT_GOAL := help

# ---------------------------------------------
# App Stack Commands
# ---------------------------------------------
app-up:
	docker compose -f $(APP_COMPOSE) up -d

app-down:
	docker compose -f $(APP_COMPOSE) down -v

app-rebuild:
	docker compose -f $(APP_COMPOSE) build --no-cache
	docker compose -f $(APP_COMPOSE) up -d

app-logs:
	docker compose -f $(APP_COMPOSE) logs -f api worker

# ---------------------------------------------
# Observability Stack Commands
# ---------------------------------------------
obs-up:
	docker compose -f $(OBS_COMPOSE) up -d

obs-down:
	docker compose -f $(OBS_COMPOSE) down -v

obs-logs:
	docker compose -f $(OBS_COMPOSE) logs -f grafana prometheus

# ---------------------------------------------
# Utility Commands
# ---------------------------------------------
request:
	curl -s -X POST localhost:8080/v1/orders \
		  -H 'Content-Type: application/json' \
		  -d '{"item":"widget","quantity":2}' >/dev/null; 

seed:
	for i in $$(seq 1 10); do \
		curl -s -X POST localhost:8080/v1/orders \
		  -H 'Content-Type: application/json' \
		  -d '{"item":"widget","quantity":2}' >/dev/null; \
	done
	@echo "✅ Seeded 10 test orders."

load:
	@read -p "Enter the number of orders [default 2000]: " O; \
	O=$${O:-2000}; \
	echo "🚀 Sending $$O requests..."; \
	seq $$O | \
	while read _; do \
		curl -s -X POST localhost:8080/v1/orders \
			-H 'Content-Type: application/json' \
			-d '{"item":"widget","quantity":2}' >/dev/null; \
		echo 1; \
	done | pv -l -s $$O -N "POST /v1/orders" >/dev/null; \
	echo "✅ Load test completed."

scale-workers:
	@read -p "Enter number of workers [default 4]: " N; \
	N=$${N:-4}; \
	docker compose -f $(APP_COMPOSE) up -d --scale worker=$$N; \
	echo "✅ Scaled workers to $$N replicas."

# ---------------------------------------------
# Housekeeping
# ---------------------------------------------
clean:
	docker compose -f $(APP_COMPOSE) down -v
	docker compose -f $(OBS_COMPOSE) down -v

help:
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "App Stack:"
	@echo "  app-up          Start app stack"
	@echo "  app-down        Stop app stack"
	@echo "  app-rebuild     Rebuild app images"
	@echo "  app-logs        Tail app logs"
	@echo ""
	@echo "Observability Stack:"
	@echo "  obs-up          Start observability stack"
	@echo "  obs-down        Stop observability stack"
	@echo "  obs-logs        Tail observability logs"
	@echo ""
	@echo "Utilities:"
	@echo "  seed            Send 10 example orders"
	@echo "  load            Run a load test (2k requests)"
	@echo "  scale-workers   Scale worker replicas"
	@echo "  clean           Remove containers and prune"
	@echo ""
