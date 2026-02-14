package handler

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newSecret(name string, data map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
		},
		"type": "Opaque",
	}}
	if data != nil {
		obj.Object["data"] = data
	}
	return obj
}

func newConfigMap(name string, data map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
		},
	}}
	if data != nil {
		obj.Object["data"] = data
	}
	return obj
}

func TestSensitiveDataFilter_SecretDataMasked(t *testing.T) {
	secret := newSecret("my-secret", map[string]interface{}{
		"password": "c2VjcmV0",
		"username": "YWRtaW4=",
	})

	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.Filter(secret)

	data, ok := result.Object["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data field to be a map")
	}

	// Keys should be preserved
	if _, exists := data["password"]; !exists {
		t.Error("expected 'password' key to be preserved")
	}
	if _, exists := data["username"]; !exists {
		t.Error("expected 'username' key to be preserved")
	}

	// Values should be masked
	if data["password"] != maskedValue {
		t.Errorf("expected password to be masked, got %v", data["password"])
	}
	if data["username"] != maskedValue {
		t.Errorf("expected username to be masked, got %v", data["username"])
	}
}

func TestSensitiveDataFilter_SecretStringDataMasked(t *testing.T) {
	secret := newSecret("my-secret", nil)
	secret.Object["stringData"] = map[string]interface{}{
		"token": "my-super-secret-token",
	}

	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.Filter(secret)

	stringData, ok := result.Object["stringData"].(map[string]interface{})
	if !ok {
		t.Fatal("expected stringData field to be a map")
	}

	if _, exists := stringData["token"]; !exists {
		t.Error("expected 'token' key to be preserved")
	}
	if stringData["token"] != maskedValue {
		t.Errorf("expected token to be masked, got %v", stringData["token"])
	}
}

func TestSensitiveDataFilter_NonSecretUnchanged(t *testing.T) {
	cm := newConfigMap("my-config", map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	})

	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.Filter(cm)

	data, ok := result.Object["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data field to be a map")
	}

	if data["key1"] != "value1" {
		t.Errorf("expected key1 to be unchanged, got %v", data["key1"])
	}
	if data["key2"] != "value2" {
		t.Errorf("expected key2 to be unchanged, got %v", data["key2"])
	}
}

func TestSensitiveDataFilter_OriginalNotModified(t *testing.T) {
	secret := newSecret("my-secret", map[string]interface{}{
		"password": "c2VjcmV0",
	})

	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	_ = filter.Filter(secret)

	// Original should be unchanged
	data := secret.Object["data"].(map[string]interface{})
	if data["password"] != "c2VjcmV0" {
		t.Errorf("original secret data was modified, got %v", data["password"])
	}
}

func TestNewSensitiveDataFilterFromParams_ShowSensitiveDataTrue(t *testing.T) {
	params := map[string]interface{}{
		"showSensitiveData": true,
	}
	filter := NewSensitiveDataFilterFromParams(params)
	if filter != nil {
		t.Error("expected nil filter when showSensitiveData is true")
	}
}

func TestNewSensitiveDataFilterFromParams_ShowSensitiveDataFalse(t *testing.T) {
	params := map[string]interface{}{
		"showSensitiveData": false,
	}
	filter := NewSensitiveDataFilterFromParams(params)
	if filter == nil {
		t.Error("expected non-nil filter when showSensitiveData is false")
	}
}

func TestNewSensitiveDataFilterFromParams_Default(t *testing.T) {
	params := map[string]interface{}{}
	filter := NewSensitiveDataFilterFromParams(params)
	if filter == nil {
		t.Error("expected non-nil filter when showSensitiveData is not set")
	}
}

func TestSensitiveDataFilter_FilterList(t *testing.T) {
	secret := newSecret("my-secret", map[string]interface{}{
		"password": "c2VjcmV0",
	})
	cm := newConfigMap("my-config", map[string]interface{}{
		"key1": "value1",
	})

	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{*secret, *cm},
	}

	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.FilterList(list)

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	// Secret data should be masked
	secretData := result.Items[0].Object["data"].(map[string]interface{})
	if secretData["password"] != maskedValue {
		t.Errorf("expected secret password to be masked, got %v", secretData["password"])
	}

	// ConfigMap data should be unchanged
	cmData := result.Items[1].Object["data"].(map[string]interface{})
	if cmData["key1"] != "value1" {
		t.Errorf("expected configmap key1 to be unchanged, got %v", cmData["key1"])
	}
}

func TestSensitiveDataFilter_NilResource(t *testing.T) {
	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.Filter(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}
}

func TestSensitiveDataFilter_NilList(t *testing.T) {
	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.FilterList(nil)
	if result != nil {
		t.Error("expected nil result for nil list input")
	}
}

func TestSensitiveDataFilter_SecretWithNoDataField(t *testing.T) {
	secret := newSecret("empty-secret", nil)

	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.Filter(secret)

	if _, exists := result.Object["data"]; exists {
		t.Error("expected no data field in result")
	}
}

func TestSensitiveDataFilter_EmptyRules(t *testing.T) {
	secret := newSecret("my-secret", map[string]interface{}{
		"password": "c2VjcmV0",
	})

	filter := NewSensitiveDataFilter([]SensitiveRule{})
	result := filter.Filter(secret)

	// Should return the original (no rules to apply)
	data := result.Object["data"].(map[string]interface{})
	if data["password"] != "c2VjcmV0" {
		t.Errorf("expected data to be unchanged with empty rules, got %v", data["password"])
	}
}

func TestSensitiveDataFilter_CaseInsensitiveKind(t *testing.T) {
	// Kind with different casing
	secret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "SECRET",
		"metadata":   map[string]interface{}{"name": "test"},
		"data": map[string]interface{}{
			"key": "value",
		},
	}}

	filter := NewSensitiveDataFilter(DefaultSensitiveRules())
	result := filter.Filter(secret)

	data := result.Object["data"].(map[string]interface{})
	if data["key"] != maskedValue {
		t.Errorf("expected case-insensitive kind match to mask data, got %v", data["key"])
	}
}
