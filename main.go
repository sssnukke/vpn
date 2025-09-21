package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Config struct {
	XrayConfigPath string `yaml:"xray_config_path"`
	ServerIP       string `yaml:"server_ip"`
	ServerPort     int    `yaml:"server_port"`
}

var config Config

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal("Error loading config:", err)
	}

	http.HandleFunc("/api/create-user", createUserHandler)
	http.HandleFunc("/api/users", listUsersHandler)
	http.HandleFunc("/api/delete-user", deleteUserHandler)

	log.Println("🚀 VLESS Manager started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadConfig() error {
	data, err := os.ReadFile("/etc/vless-manager/config.yaml")
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &config)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct{ Email string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	vlessLink, userID, err := createVlessUser(req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"vless_link": vlessLink,
		"user_id":    userID,
	})
}

func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users, err := getXrayUsers()
	if err != nil {
		http.Error(w, "Error getting users: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if err := deleteXrayUser(userID); err != nil {
		http.Error(w, "Error deleting user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := reloadXray(); err != nil {
		http.Error(w, "Error reloading Xray: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "user deleted"})
}

func createVlessUser(email string) (string, string, error) {
	userUUID := uuid.New().String()

	if err := updateXrayConfig(userUUID, email); err != nil {
		return "", "", err
	}

	if err := reloadXray(); err != nil {
		return "", "", err
	}

	vlessLink := fmt.Sprintf("vless://%s@%s:%d?encryption=none&flow=xtls-rprx-vision&security=tls&type=tcp#%s",
		userUUID, config.ServerIP, config.ServerPort, email)

	return vlessLink, userUUID, nil
}

func updateXrayConfig(userUUID, email string) error {
	data, err := os.ReadFile(config.XrayConfigPath)
	if err != nil {
		return err
	}

	var xrayConfig map[string]interface{}
	if err := json.Unmarshal(data, &xrayConfig); err != nil {
		return err
	}

	inbounds := xrayConfig["inbounds"].([]interface{})
	inbound := inbounds[0].(map[string]interface{})
	settings := inbound["settings"].(map[string]interface{})
	clients := settings["clients"].([]interface{})

	newClient := map[string]interface{}{
		"id":    userUUID,
		"email": email,
		"flow":  "xtls-rprx-vision",
	}
	clients = append(clients, newClient)
	settings["clients"] = clients

	updatedData, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(config.XrayConfigPath, updatedData, 0644)
}

func getXrayUsers() ([]map[string]interface{}, error) {
	data, err := os.ReadFile(config.XrayConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error reading Xray config: %v", err)
	}

	var xrayConfig map[string]interface{}
	if err := json.Unmarshal(data, &xrayConfig); err != nil {
		return nil, fmt.Errorf("error parsing Xray config: %v", err)
	}

	inbounds, ok := xrayConfig["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		return []map[string]interface{}{}, nil
	}

	inbound, ok := inbounds[0].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	settings, ok := inbound["settings"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	clients, ok := settings["clients"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	var users []map[string]interface{}
	for _, client := range clients {
		if clientMap, ok := client.(map[string]interface{}); ok {
			users = append(users, clientMap)
		}
	}

	return users, nil
}

func deleteXrayUser(userID string) error {
	data, err := os.ReadFile(config.XrayConfigPath)
	if err != nil {
		return fmt.Errorf("error reading config: %v", err)
	}

	var xrayConfig map[string]interface{}
	if err := json.Unmarshal(data, &xrayConfig); err != nil {
		return fmt.Errorf("error parsing config: %v", err)
	}

	inbounds := xrayConfig["inbounds"].([]interface{})
	inbound := inbounds[0].(map[string]interface{})
	settings := inbound["settings"].(map[string]interface{})
	clients := settings["clients"].([]interface{})

	var newClients []interface{}
	for _, client := range clients {
		if clientMap, ok := client.(map[string]interface{}); ok {
			if clientID, exists := clientMap["id"]; exists && clientID != userID {
				newClients = append(newClients, client)
			}
		}
	}

	settings["clients"] = newClients
	updatedData, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config: %v", err)
	}

	return os.WriteFile(config.XrayConfigPath, updatedData, 0644)
}

func reloadXray() error {
	cmd := exec.Command("sudo", "systemctl", "reload", "xray")
	return cmd.Run()
}
