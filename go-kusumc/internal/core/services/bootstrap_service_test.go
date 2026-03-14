package services

import (
	"os"
	"reflect"
	"testing"
)

type mockBootstrapRepo struct {
	device map[string]interface{}
	cred   map[string]interface{}
}

func (m *mockBootstrapRepo) GetDeviceByIMEI(imei string) (interface{}, error) {
	return m.device, nil
}

func (m *mockBootstrapRepo) GetDeviceByID(id string) (map[string]interface{}, error) { return nil, nil }
func (m *mockBootstrapRepo) GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockBootstrapRepo) ListDevices(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error) {
	return nil, 0, nil
}
func (m *mockBootstrapRepo) UpdateDeviceByIDOrIMEI(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockBootstrapRepo) CreateDeviceStruct(projectId, name, imei string, mqttBundle map[string]interface{}, attrs map[string]interface{}) (string, error) {
	return "", nil
}
func (m *mockBootstrapRepo) SoftDeleteDevice(idOrIMEI string) error { return nil }
func (m *mockBootstrapRepo) InsertCredentialHistory(deviceID string, bundle map[string]interface{}) (string, error) {
	return "", nil
}
func (m *mockBootstrapRepo) GetAutomationFlow(projectId string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockBootstrapRepo) GetInstallationByDevice(deviceId string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockBootstrapRepo) GetBeneficiary(id string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockBootstrapRepo) CreateMqttProvisioningJob(deviceId string, credHistoryId *string) error {
	return nil
}

func (m *mockBootstrapRepo) GetLatestCredentialHistory(deviceId string) (map[string]interface{}, error) {
	return m.cred, nil
}

func (m *mockBootstrapRepo) ListCredentialHistory(deviceId string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *mockBootstrapRepo) GetPendingCommands(deviceId string) ([]map[string]interface{}, error) {
	return nil, nil
}

func TestBootstrapUsesPublicDefaultsWhenNoProtocolOrBundle(t *testing.T) {
	setEnv(t, "MQTT_HOST", "emqx-internal")
	setEnv(t, "MQTT_PORT", "1883")
	setEnv(t, "MQTT_PUBLIC_HOST", "iot.local")
	setEnv(t, "MQTT_PUBLIC_PORT", "8889")
	setEnv(t, "MQTT_PUBLIC_PROTOCOL", "mqtts")

	repo := &mockBootstrapRepo{
		device: map[string]interface{}{
			"id":         "dev-1",
			"imei":       "123",
			"project_id": "proj-1",
		},
		cred: map[string]interface{}{},
	}

	service := NewBootstrapService(repo, nil, nil, nil, nil)
	cfg, err := service.GetDeviceConfig("123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	primary, ok := cfg["primary_broker"].(map[string]interface{})
	if !ok {
		t.Fatalf("primary_broker missing or wrong type: %T", cfg["primary_broker"])
	}

	host := primary["host"]
	if host != "iot.local" {
		t.Fatalf("expected host iot.local, got %v", host)
	}

	port := primary["port"]
	if port != "8889" {
		t.Fatalf("expected port 8889, got %v", port)
	}

	endpoints := toStringSlice(primary["endpoints"])
	expectedEndpoint := "mqtts://iot.local:8889"
	if len(endpoints) == 0 || endpoints[0] != expectedEndpoint {
		t.Fatalf("expected endpoint %s, got %v", expectedEndpoint, endpoints)
	}
}

func toStringSlice(v interface{}) []string {
	switch raw := v.(type) {
	case []string:
		return raw
	case []interface{}:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func setEnv(t *testing.T, key, val string) {
	t.Helper()
	prev, existed := os.LookupEnv(key)
	os.Setenv(key, val)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	})
}

// Ensure mock implements required interface
var _ interface {
	GetDeviceByIMEI(string) (interface{}, error)
	GetDeviceByID(string) (map[string]interface{}, error)
	GetDeviceByIDOrIMEI(string) (map[string]interface{}, error)
	SoftDeleteDevice(string) error
	CreateDeviceStruct(string, string, string, map[string]interface{}, map[string]interface{}) (string, error)
	InsertCredentialHistory(string, map[string]interface{}) (string, error)
	GetAutomationFlow(string) (map[string]interface{}, error)
	GetInstallationByDevice(string) (map[string]interface{}, error)
	GetBeneficiary(string) (map[string]interface{}, error)
	CreateMqttProvisioningJob(string, *string) error
	GetLatestCredentialHistory(string) (map[string]interface{}, error)
	ListCredentialHistory(string) ([]map[string]interface{}, error)
	GetPendingCommands(string) ([]map[string]interface{}, error)
} = (*mockBootstrapRepo)(nil)

// Guard against accidental field changes in primary_broker
func TestPrimaryBrokerShapeStable(t *testing.T) {
	repo := &mockBootstrapRepo{
		device: map[string]interface{}{"id": "dev", "imei": "123", "project_id": "proj"},
	}
	service := NewBootstrapService(repo, nil, nil, nil, nil)
	cfg, err := service.GetDeviceConfig("123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	primary, ok := cfg["primary_broker"].(map[string]interface{})
	if !ok {
		t.Fatalf("primary_broker type mismatch: %T", cfg["primary_broker"])
	}

	keys := make([]string, 0, len(primary))
	for k := range primary {
		keys = append(keys, k)
	}

	expected := []string{"protocol", "protocol_id", "host", "port", "username", "password", "client_id", "publish_topics", "subscribe_topics", "endpoints"}
	if !reflect.DeepEqual(sorted(keys), sorted(expected)) {
		t.Fatalf("primary_broker keys changed; got %v expected %v", keys, expected)
	}
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
