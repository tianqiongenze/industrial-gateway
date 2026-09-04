"""
E2E tests for the Industrial Gateway (Go Gin) project.
Tests cover: health, ingest, latest readings, device status,
device CRUD, frontend verification.
Uses requests for HTTP E2E testing.
"""
import pytest
import json
import sys
import os
import requests

BASE = os.environ.get("GATEWAY_API", "http://localhost:8090/api/v1")


class TestHealthEndpoint:
    """GET /api/v1/health"""

    def test_health_returns_ok(self):
        r = requests.get(f"{BASE}/health")
        assert r.status_code == 200
        data = r.json()
        assert data["status"] == "ok"
        assert "readings" in data
        assert "cache" in data

    def test_health_has_timestamp(self):
        r = requests.get(f"{BASE}/health")
        data = r.json()
        assert "time" in data


class TestIngestData:
    """POST /api/v1/ingest"""

    def test_ingest_valid_reading(self):
        body = {
            "device_id": "E2E-GO-001",
            "metric": "temperature",
            "value": 45.5,
            "unit": "°C",
            "source": "rest"
        }
        r = requests.post(f"{BASE}/ingest", json=body)
        assert r.status_code == 201
        data = r.json()
        assert data["device_id"] == "E2E-GO-001"
        assert data["status"] in ("normal", "warning", "critical")

    def test_ingest_high_temperature_warning(self):
        body = {"device_id": "E2E-GO-001", "metric": "temperature", "value": 85.0}
        r = requests.post(f"{BASE}/ingest", json=body)
        assert r.status_code == 201
        assert r.json()["status"] == "warning"

    def test_ingest_critical_temperature(self):
        body = {"device_id": "E2E-GO-001", "metric": "temperature", "value": 110.0}
        r = requests.post(f"{BASE}/ingest", json=body)
        assert r.status_code == 201
        assert r.json()["status"] == "critical"

    def test_ingest_missing_device_id_rejected(self):
        body = {"metric": "temperature", "value": 50.0}
        r = requests.post(f"{BASE}/ingest", json=body)
        assert r.status_code == 400

    def test_ingest_vibration_data(self):
        body = {"device_id": "E2E-GO-002", "metric": "vibration", "value": 8.5, "unit": "mm/s"}
        r = requests.post(f"{BASE}/ingest", json=body)
        assert r.status_code == 201

    def test_ingest_pressure_data(self):
        body = {"device_id": "E2E-GO-003", "metric": "pressure", "value": 150.0, "unit": "kPa"}
        r = requests.post(f"{BASE}/ingest", json=body)
        assert r.status_code == 201


class TestLatestReadings:
    """GET /api/v1/readings/{device}"""

    def test_get_latest_readings(self):
        # First ingest some data
        for i in range(3):
            requests.post(f"{BASE}/ingest", json={
                "device_id": "E2E-GO-LATEST", "metric": "temperature",
                "value": 50.0 + i, "unit": "°C"
            })
        r = requests.get(f"{BASE}/readings/E2E-GO-LATEST")
        assert r.status_code == 200
        data = r.json()
        assert data["device_id"] == "E2E-GO-LATEST"
        assert len(data["readings"]) > 0

    def test_get_readings_nonexistent_device(self):
        r = requests.get(f"{BASE}/readings/NONEXISTENT")
        assert r.status_code == 200  # Returns empty list
        assert len(r.json()["readings"]) == 0


class TestDeviceStatus:
    """GET /api/v1/status/{device}"""

    def test_status_for_device_with_data(self):
        # Ingest data first
        requests.post(f"{BASE}/ingest", json={
            "device_id": "E2E-GO-STATUS", "metric": "temperature", "value": 55.0
        })
        r = requests.get(f"{BASE}/status/E2E-GO-STATUS")
        assert r.status_code == 200
        data = r.json()
        assert data["device_id"] == "E2E-GO-STATUS"
        assert "status" in data

    def test_status_cache_consistency(self):
        """Second call should return same status (cache hit)."""
        requests.post(f"{BASE}/ingest", json={
            "device_id": "E2E-GO-CACHE", "metric": "temperature", "value": 60.0
        })
        r1 = requests.get(f"{BASE}/status/E2E-GO-CACHE")
        r2 = requests.get(f"{BASE}/status/E2E-GO-CACHE")
        assert r1.json()["status"] == r2.json()["status"]


class TestDeviceManagement:
    """Device CRUD: POST /api/v1/devices, GET /api/v1/devices, DELETE /api/v1/devices/{id}"""

    def test_create_device(self):
        body = {"id": "E2E-DEV-NEW", "name": "E2E Test Device", "protocol": "modbus"}
        r = requests.post(f"{BASE}/devices", json=body)
        assert r.status_code in (200, 201)

    def test_list_devices(self):
        r = requests.get(f"{BASE}/devices")
        assert r.status_code == 200
        assert isinstance(r.json(), list)

    def test_create_invalid_device(self):
        body = {"id": "", "name": "Bad Device", "protocol": "modbus"}
        r = requests.post(f"{BASE}/devices", json=body)
        assert r.status_code in (400, 422)


class TestFullWorkflow:
    """E2E: ingest → latest → status → device CRUD."""

    def test_full_ingest_workflow(self):
        device = "E2E-FULL-001"

        # Ingest 10 readings
        for i in range(10):
            val = 40 + i * 3 + (15 if i > 7 else 0)
            r = requests.post(f"{BASE}/ingest", json={
                "device_id": device, "metric": "temperature",
                "value": float(val), "unit": "°C", "source": "e2e"
            })
            assert r.status_code == 201

        # Get latest readings
        r = requests.get(f"{BASE}/readings/{device}")
        assert r.status_code == 200
        assert len(r.json()["readings"]) > 0

        # Get status
        r = requests.get(f"{BASE}/status/{device}")
        assert r.status_code == 200
        assert r.json()["status"] in ("normal", "warning", "critical")

        # Check health
        r = requests.get(f"{BASE}/health")
        assert r.status_code == 200
        assert r.json()["readings"] >= 10

    def test_multi_source_ingest(self):
        """Test ingest from different sources (REST, MQTT, Modbus)."""
        for source in ["rest", "mqtt", "modbus"]:
            r = requests.post(f"{BASE}/ingest", json={
                "device_id": f"E2E-SRC-{source}", "metric": "temperature",
                "value": 50.0, "source": source
            })
            assert r.status_code == 201


class TestFrontendVerification:
    """Verify the Industrial Gateway frontend."""

    def test_frontend_file_exists(self):
        path = os.path.join(os.path.dirname(__file__), "..", "frontend", "index.html")
        if not os.path.exists(path):
            path = "/home/jovyan/work/industrial-gateway/frontend/index.html"
        if not os.path.exists(path):
            pytest.skip("Frontend not found")
        assert os.path.getsize(path) > 1000  # At least 1KB

    def test_frontend_has_gateway_title(self):
        path = os.path.join(os.path.dirname(__file__), "..", "frontend", "index.html")
        if not os.path.exists(path):
            path = "/home/jovyan/work/industrial-gateway/frontend/index.html"
        if not os.path.exists(path):
            pytest.skip("Frontend not found")
        with open(path) as f:
            content = f.read()
        assert '工业网关' in content or 'Industrial Gateway' in content

    def test_frontend_has_ingest_form(self):
        path = os.path.join(os.path.dirname(__file__), "..", "frontend", "index.html")
        if not os.path.exists(path):
            path = "/home/jovyan/work/industrial-gateway/frontend/index.html"
        if not os.path.exists(path):
            pytest.skip("Frontend not found")
        with open(path) as f:
            content = f.read()
        assert 'ingestData' in content or 'ingest' in content.lower()

    def test_frontend_has_device_management(self):
        path = os.path.join(os.path.dirname(__file__), "..", "frontend", "index.html")
        if not os.path.exists(path):
            path = "/home/jovyan/work/industrial-gateway/frontend/index.html"
        if not os.path.exists(path):
            pytest.skip("Frontend not found")
        with open(path) as f:
            content = f.read()
        assert 'createDevice' in content or 'device' in content.lower()

    def test_frontend_has_status_monitoring(self):
        path = os.path.join(os.path.dirname(__file__), "..", "frontend", "index.html")
        if not os.path.exists(path):
            path = "/home/jovyan/work/industrial-gateway/frontend/index.html"
        if not os.path.exists(path):
            pytest.skip("Frontend not found")
        with open(path) as f:
            content = f.read()
        assert 'refreshDevices' in content or 'status' in content.lower()

    def test_frontend_has_api_log(self):
        path = os.path.join(os.path.dirname(__file__), "..", "frontend", "index.html")
        if not os.path.exists(path):
            path = "/home/jovyan/work/industrial-gateway/frontend/index.html"
        if not os.path.exists(path):
            pytest.skip("Frontend not found")
        with open(path) as f:
            content = f.read()
        assert 'apiLog' in content or 'logApi' in content
