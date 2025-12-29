.PHONY: help build run test docker-build docker-run k8s-deploy k8s-delete load-test clean

# Переменные
BINARY_NAME=highload-service
DOCKER_IMAGE=highload-service:latest
GO_FILES=$(shell find . -name '*.go' -type f)

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Собрать бинарник
	@echo "🔨 Сборка..."
	go build -o $(BINARY_NAME) .
	@echo "✅ Сборка завершена: $(BINARY_NAME)"

run: ## Запустить локально
	@echo "🚀 Запуск сервиса..."
	go run main.go

test: ## Запустить тесты
	@echo "🧪 Запуск тестов..."
	go test -v ./...

test-coverage: ## Тесты с покрытием
	@echo "📊 Тесты с покрытием..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Отчет: coverage.html"

lint: ## Проверка кода
	@echo "🔍 Линтинг..."
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint не установлен"; \
	fi

docker-build: ## Собрать Docker образ
	@echo "🐳 Сборка Docker образа..."
	docker build -t $(DOCKER_IMAGE) .
	@echo "✅ Образ собран: $(DOCKER_IMAGE)"

docker-run: docker-build ## Запустить в Docker
	@echo "🐳 Запуск в Docker..."
	docker run -p 8080:8080 --rm \
		-e REDIS_ADDR=host.docker.internal:6379 \
		$(DOCKER_IMAGE)

docker-compose-up: ## Запустить docker-compose
	@echo "🐳 Запуск docker-compose..."
	docker-compose up -d
	@echo "✅ Сервисы запущены"
	@echo "   API: http://localhost:8080"
	@echo "   Prometheus: http://localhost:9090"
	@echo "   Grafana: http://localhost:3000"

docker-compose-down: ## Остановить docker-compose
	@echo "🛑 Остановка docker-compose..."
	docker-compose down

docker-compose-logs: ## Показать логи docker-compose
	docker-compose logs -f

k8s-deploy: docker-build ## Развернуть в Kubernetes
	@echo "☸️  Развертывание в Kubernetes..."
	@if command -v minikube >/dev/null 2>&1; then \
		eval $$(minikube docker-env); \
		docker build -t $(DOCKER_IMAGE) .; \
	fi
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/redis.yaml
	kubectl apply -f k8s/deployment.yaml
	kubectl apply -f k8s/hpa.yaml
	@echo "✅ Развертывание завершено"

k8s-delete: ## Удалить из Kubernetes
	@echo "🗑️  Удаление из Kubernetes..."
	kubectl delete -f k8s/ || true
	@echo "✅ Удалено"

k8s-status: ## Статус в Kubernetes
	@echo "📊 Статус подов:"
	kubectl get pods
	@echo ""
	@echo "📊 Статус сервисов:"
	kubectl get svc
	@echo ""
	@echo "📊 Статус HPA:"
	kubectl get hpa

k8s-logs: ## Логи из Kubernetes
	kubectl logs -f deployment/highload-service

load-test-ab: ## Нагрузочный тест Apache Bench
	@echo "🔥 Запуск нагрузочного теста (Apache Bench)..."
	@if [ -f tests/load/load-test.sh ]; then \
		chmod +x tests/load/load-test.sh; \
		./tests/load/load-test.sh localhost 8080 10000 100; \
	else \
		echo "❌ Файл tests/load/load-test.sh не найден"; \
	fi

load-test-locust: ## Нагрузочный тест Locust
	@echo "🔥 Запуск Locust..."
	@if command -v locust >/dev/null 2>&1; then \
		cd tests/load && locust -f locustfile.py --host=http://localhost:8080; \
	else \
		echo "❌ Locust не установлен. Установите: pip install locust"; \
	fi

simulate-iot: ## Симуляция IoT устройств
	@echo "📡 Симуляция IoT устройств..."
	@chmod +x tests/load/simulate-iot.sh
	./tests/load/simulate-iot.sh localhost:8080 10 60

deps: ## Установить зависимости
	@echo "📦 Установка зависимостей..."
	go mod download
	go mod tidy

clean: ## Очистка
	@echo "🧹 Очистка..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	@echo "✅ Очищено"

dev-setup: deps ## Настройка окружения разработки
	@echo "🛠️  Настройка окружения..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "📥 Установка golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "✅ Окружение готово"

.DEFAULT_GOAL := help

