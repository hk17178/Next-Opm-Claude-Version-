package biz

import (
	"testing"
)

// --- inferAssetType Tests ---

func TestInferAssetType_NodeExporter(t *testing.T) {
	labels := map[string]string{"job": "node-exporter"}
	if got := inferAssetType(labels); got != AssetTypeServer {
		t.Errorf("inferAssetType(node-exporter) = %s, want %s", got, AssetTypeServer)
	}
}

func TestInferAssetType_MySQL(t *testing.T) {
	labels := map[string]string{"job": "mysql-exporter"}
	if got := inferAssetType(labels); got != AssetTypeDatabase {
		t.Errorf("inferAssetType(mysql-exporter) = %s, want %s", got, AssetTypeDatabase)
	}
}

func TestInferAssetType_Postgres(t *testing.T) {
	labels := map[string]string{"job": "postgres-exporter"}
	if got := inferAssetType(labels); got != AssetTypeDatabase {
		t.Errorf("inferAssetType(postgres-exporter) = %s, want %s", got, AssetTypeDatabase)
	}
}

func TestInferAssetType_Nginx(t *testing.T) {
	labels := map[string]string{"job": "nginx"}
	if got := inferAssetType(labels); got != AssetTypeLoadBalancer {
		t.Errorf("inferAssetType(nginx) = %s, want %s", got, AssetTypeLoadBalancer)
	}
}

func TestInferAssetType_Kubernetes(t *testing.T) {
	labels := map[string]string{"job": "kube-state-metrics"}
	if got := inferAssetType(labels); got != AssetTypeK8sCluster {
		t.Errorf("inferAssetType(kube-state-metrics) = %s, want %s", got, AssetTypeK8sCluster)
	}
}

func TestInferAssetType_Kafka(t *testing.T) {
	labels := map[string]string{"job": "kafka-exporter"}
	if got := inferAssetType(labels); got != AssetTypeMessageQueue {
		t.Errorf("inferAssetType(kafka-exporter) = %s, want %s", got, AssetTypeMessageQueue)
	}
}

func TestInferAssetType_Unknown(t *testing.T) {
	labels := map[string]string{"job": "custom-app"}
	if got := inferAssetType(labels); got != AssetTypeServer {
		t.Errorf("inferAssetType(unknown) = %s, want %s (default)", got, AssetTypeServer)
	}
}

func TestInferAssetType_NoJob(t *testing.T) {
	labels := map[string]string{"instance": "10.0.0.1:9090"}
	if got := inferAssetType(labels); got != AssetTypeServer {
		t.Errorf("inferAssetType(no job) = %s, want %s (default)", got, AssetTypeServer)
	}
}

// --- splitHostPort Tests ---

func TestSplitHostPort_WithPort(t *testing.T) {
	if got := splitHostPort("10.0.0.1:9090"); got != "10.0.0.1" {
		t.Errorf("splitHostPort = %s, want 10.0.0.1", got)
	}
}

func TestSplitHostPort_WithoutPort(t *testing.T) {
	if got := splitHostPort("10.0.0.1"); got != "10.0.0.1" {
		t.Errorf("splitHostPort = %s, want 10.0.0.1", got)
	}
}

func TestSplitHostPort_Hostname(t *testing.T) {
	if got := splitHostPort("web-01:8080"); got != "web-01" {
		t.Errorf("splitHostPort = %s, want web-01", got)
	}
}

// --- strPtr Tests ---

func TestStrPtr_NonEmpty(t *testing.T) {
	p := strPtr("hello")
	if p == nil || *p != "hello" {
		t.Errorf("strPtr(hello) = %v, want &hello", p)
	}
}

func TestStrPtr_Empty(t *testing.T) {
	p := strPtr("")
	if p != nil {
		t.Error("strPtr(\"\") should return nil")
	}
}
