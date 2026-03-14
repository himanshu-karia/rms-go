package http

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"

	"ingestion-go/internal/core/services"
)

type mockDeviceRepo struct {
	device map[string]interface{}
	cred   map[string]interface{}
}

func (m *mockDeviceRepo) GetDeviceByIMEI(imei string) (interface{}, error)        { return m.device, nil }
func (m *mockDeviceRepo) GetDeviceByID(id string) (map[string]interface{}, error) { return nil, nil }
func (m *mockDeviceRepo) GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) ListDevices(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error) {
	return nil, 0, nil
}
func (m *mockDeviceRepo) UpdateDeviceByIDOrIMEI(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) CreateDeviceStruct(projectId, name, imei string, mqttBundle map[string]interface{}, attrs map[string]interface{}) (string, error) {
	return "", nil
}
func (m *mockDeviceRepo) SoftDeleteDevice(idOrIMEI string) error {
	return nil
}
func (m *mockDeviceRepo) InsertCredentialHistory(deviceID string, bundle map[string]interface{}) (string, error) {
	return "", nil
}
func (m *mockDeviceRepo) GetAutomationFlow(projectId string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) GetInstallationByDevice(deviceId string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) GetBeneficiary(id string) (map[string]interface{}, error) { return nil, nil }
func (m *mockDeviceRepo) CreateMqttProvisioningJob(deviceId string, credHistoryId *string) error {
	return nil
}
func (m *mockDeviceRepo) GetLatestCredentialHistory(deviceId string) (map[string]interface{}, error) {
	return m.cred, nil
}
func (m *mockDeviceRepo) ListCredentialHistory(deviceId string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *mockDeviceRepo) GetPendingCommands(deviceId string) ([]map[string]interface{}, error) {
	return nil, nil
}

func TestBootstrapRouteReturnsTLSDefaults(t *testing.T) {
	setEnv(t, "MQTT_PUBLIC_HOST", "iot.local")
	setEnv(t, "MQTT_PUBLIC_PORT", "8889")
	setEnv(t, "MQTT_PUBLIC_PROTOCOL", "mqtts")

	repo := &mockDeviceRepo{device: map[string]interface{}{"id": "dev", "imei": "abc", "project_id": "proj"}}
	boot := services.NewBootstrapService(repo, nil, nil, nil, nil)

	app := fiber.New()
	router := NewRouter(nil, nil, boot, nil, nil, nil, nil, nil, nil, nil)
	public := app.Group("/api")
	protected := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("auth_method", "api_key")
		c.Locals("project_id", "proj")
		return c.Next()
	})
	router.RegisterRoutes(public, protected)

	req := httptest.NewRequest("GET", "/api/bootstrap?imei=abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	pb, ok := payload["primary_broker"].(map[string]interface{})
	if !ok {
		t.Fatalf("primary_broker missing or wrong type: %T", payload["primary_broker"])
	}

	endpoints := toStringSlice(pb["endpoints"])
	if len(endpoints) == 0 || endpoints[0] != "mqtts://iot.local:8889" {
		t.Fatalf("unexpected endpoints: %v", endpoints)
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

// Ensure mock satisfies the DeviceRepo shape expected by BootstrapService
var _ interface {
	GetDeviceByIMEI(string) (interface{}, error)
	GetInstallationByDevice(string) (map[string]interface{}, error)
	GetBeneficiary(string) (map[string]interface{}, error)
	CreateMqttProvisioningJob(string, *string) error
	GetLatestCredentialHistory(string) (map[string]interface{}, error)
	ListCredentialHistory(string) ([]map[string]interface{}, error)
	GetPendingCommands(string) ([]map[string]interface{}, error)
} = (*mockDeviceRepo)(nil)
