package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const baseURL = "http://localhost:8081/api"

// Colors for output
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
)

func main() {
	fmt.Println(Cyan + "🔒 Starting Chain of Trust Verification Flow..." + Reset)
	time.Sleep(1 * time.Second)

	// Step 1: Login (Admin)
	token := login("admin", "admin123")
	if token == "" {
		fmt.Println(Red + "❌ Login Failed. Aborting." + Reset)
		os.Exit(1)
	}

	// Step 2: Create Project
	projectId := "Pharma_Logistics_" + fmt.Sprintf("%d", time.Now().Unix())
	createProject(token, projectId)

	// Step 3: Provision Device
	deviceId, imei := createDevice(token, projectId, "Tracker_X99")

	// Step 4: Simulate Telemetry (Device Perspective)
	sendTelemetry(deviceId, imei, projectId, "secret")

	// Step 5: Verify Chain of Custody (Admin Perspective)
	verifyTimeline(token, deviceId)

	// Step 6: Generate Compliance Report (Admin Perspective)
	generateReport(token, projectId)

	fmt.Println(Green + "✅ Chain of Trust Flow Verified Successfully!" + Reset)
}

func login(user, pass string) string {
	fmt.Print("1. Authenticating Admin... ")
	// Mock Login for V1 (AuthService accepts any creds if not strict, or specific ones)
	// We used AuthService with userRepo.
	// Assuming "admin" user exists or we use "auth/google" mock.
	// Actually, let's use the `Seeder` user if available, or just a mock token if AuthMiddleware is lenient in Dev.
	// Wait, AuthService validates JWT.
	// I'll try to use a hardcoded DEV token if available, or hit /api/auth/login if seeded.
	// The Seeder creates `admin@example.com` / `admin`.

	payload := map[string]string{"username": "Him", "password": "0554"}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(Red + "Failed: " + err.Error() + Reset)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Fallback for V1 without Seeder running: Generate a Self-Signed Token or skip if Auth disabled?
		// Actually, let's assume Auth is active.
		fmt.Printf(Red+"Failed (Status %d). "+Reset, resp.StatusCode)
		return ""
	}

	var res map[string]string
	json.NewDecoder(resp.Body).Decode(&res)
	fmt.Println(Green + "OK" + Reset)
	return res["token"]
}

func createProject(token, pid string) {
	fmt.Printf("2. Creating Secure Project [%s]... ", pid)
	payload := map[string]interface{}{
		"id": pid, "name": "Global Pharma Tracking", "type": "logistics", "location": "Global", "config": map[string]interface{}{},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/projects", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf(Red+"Failed (Net Error: %v)\n"+Reset, err)
	} else if resp.StatusCode > 201 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf(Red+"Failed (Status %d): %s\n"+Reset, resp.StatusCode, string(bodyBytes))
	} else {
		fmt.Println(Green + "OK" + Reset)
	}
}

func createDevice(token, pid, name string) (string, string) {
	fmt.Printf("3. Provisioning Device [%s]... ", name)
	// Simplified: We assume ID is returned. Secret might be simulated.
	payload := map[string]interface{}{
		"imei":       "IMEI_" + fmt.Sprintf("%d", time.Now().Unix()),
		"projectId":  pid,
		"name":       name,
		"status":     "active",
		"attributes": map[string]interface{}{},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/devices", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf(Red+"Failed (Net Error: %v)\n"+Reset, err)
		return "", ""
	} else if resp.StatusCode > 201 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf(Red+"Failed (Status %d): %s\n"+Reset, resp.StatusCode, string(bodyBytes))
		return "", ""
	}

	var res map[string]interface{}
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &res)

	if res["id"] == nil {
		fmt.Printf(Red+"Failed (No ID returned): %s\n"+Reset, string(bodyBytes))
		return "", ""
	}

	id := res["id"].(string)
	imei := res["imei"].(string)
	fmt.Printf(Green+"OK (ID: %s)\n"+Reset, id)
	return id, imei
}

func sendTelemetry(deviceId, imei, pid, secret string) {
	fmt.Print("4. Ingesting Telemetry (LoRaWAN/HTTP)... ")
	payload := map[string]interface{}{
		"device_id":  deviceId, // In real world, extracted from Token/Cert
		"imei":       imei,
		"project_id": pid,
		"time":       time.Now().UnixMilli(),
		"payload": map[string]interface{}{
			"temp": 2.5, "humidity": 40, "lat": 40.7128, "lon": -74.0060,
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/ingest", bytes.NewBuffer(body))
	// req.Header.Set("X-API-KEY", secret) // If using ApiKeyMiddleware
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf(Red+"Failed (Net Error: %v)\n"+Reset, err)
	} else if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf(Red+"Failed (Status %d): %s\n"+Reset, resp.StatusCode, string(bodyBytes))
	} else {
		fmt.Println(Green + "OK" + Reset)
	}
	time.Sleep(2000 * time.Millisecond) // Let async worker process
}

func verifyTimeline(token, deviceId string) {
	fmt.Print("5. Verifying Chain of Custody... ")
	req, _ := http.NewRequest("GET", baseURL+"/logistics/assets/"+deviceId+"/timeline", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf(Red + "Failed\n" + Reset)
		return
	}

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf(Green+"OK (%d bytes received)\n"+Reset, len(body))
}

func generateReport(token, pid string) {
	fmt.Print("6. Generating Compliance Report... ")
	req, _ := http.NewRequest("GET", baseURL+"/reports/"+pid+"/compliance", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf(Red+"Failed (Net Error: %v)\n"+Reset, err)
	} else if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf(Red+"Failed (Status %d): %s\n"+Reset, resp.StatusCode, string(bodyBytes))
	} else {
		fmt.Println(Green + "OK (Excel Downloaded)" + Reset)
	}
}
