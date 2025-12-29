from locust import HttpUser, task, between
import random
import time
from datetime import datetime

class IoTMetricsUser(HttpUser):
    """
    Симуляция нагрузки от IoT устройств
    """
    wait_time = between(0.01, 0.1)  # Интервал между запросами 10-100мс

    def on_start(self):
        """Инициализация при старте пользователя"""
        self.device_id = f"device-{random.randint(1, 100)}"
        self.base_cpu = random.uniform(20, 80)
        self.base_rps = random.uniform(50, 200)

    @task(10)
    def submit_normal_metric(self):
        """Отправка нормальной метрики (10/11 запросов)"""
        cpu = self.base_cpu + random.gauss(0, 5)
        rps = self.base_rps + random.gauss(0, 10)

        metric = {
            "device_id": self.device_id,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "cpu": max(0, min(100, cpu)),
            "rps": max(0, rps)
        }

        self.client.post("/metrics", json=metric, name="/metrics (normal)")

    @task(1)
    def submit_anomaly_metric(self):
        """Отправка аномальной метрики (1/11 запросов)"""
        # Создаем аномалию: CPU spike или RPS spike
        if random.random() > 0.5:
            cpu = self.base_cpu + random.uniform(30, 50)  # CPU spike
            rps = self.base_rps + random.gauss(0, 10)
        else:
            cpu = self.base_cpu + random.gauss(0, 5)
            rps = self.base_rps + random.uniform(100, 200)  # RPS spike

        metric = {
            "device_id": self.device_id,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "cpu": max(0, min(100, cpu)),
            "rps": max(0, rps)
        }

        self.client.post("/metrics", json=metric, name="/metrics (anomaly)")

    @task(2)
    def submit_batch_metrics(self):
        """Отправка пакета метрик"""
        batch_size = random.randint(5, 20)
        batch = []

        for _ in range(batch_size):
            cpu = self.base_cpu + random.gauss(0, 5)
            rps = self.base_rps + random.gauss(0, 10)

            batch.append({
                "device_id": self.device_id,
                "timestamp": datetime.utcnow().isoformat() + "Z",
                "cpu": max(0, min(100, cpu)),
                "rps": max(0, rps)
            })

        self.client.post("/metrics/batch", json=batch, name="/metrics/batch")

    @task(1)
    def check_analytics(self):
        """Проверка аналитики для устройства"""
        self.client.get(f"/analytics?device_id={self.device_id}", name="/analytics")

    @task(1)
    def check_health(self):
        """Проверка здоровья сервиса"""
        self.client.get("/health", name="/health")

    @task(1)
    def check_stats(self):
        """Проверка статистики"""
        self.client.get("/stats", name="/stats")


class HighLoadUser(HttpUser):
    """
    Высоконагруженный пользователь для stress-тестов
    """
    wait_time = between(0.001, 0.01)  # Очень короткий интервал

    def on_start(self):
        self.device_id = f"device-stress-{random.randint(1, 1000)}"

    @task
    def rapid_fire_metrics(self):
        """Быстрая отправка метрик"""
        metric = {
            "device_id": self.device_id,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "cpu": random.uniform(0, 100),
            "rps": random.uniform(0, 500)
        }

        self.client.post("/metrics", json=metric, name="/metrics (rapid)")

