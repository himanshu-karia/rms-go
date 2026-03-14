package config

import (
	"log"
	"os"
)

type Config struct {
	MqttBroker   string
	RedisAddr    string
	TimescaleURI string
	ServerPort   string
}

func Load() *Config {
	return &Config{
		MqttBroker:   getEnv("MQTT_URL", "tcp://localhost:1883"),
		RedisAddr:    getEnv("REDIS_URL", "localhost:6379"), // Configured for standard connection string parsing in Repo
		TimescaleURI: getEnv("TIMESCALE_URI", "postgres://user:pass@localhost:5432/db"),
		ServerPort:   getEnv("GO_PORT", "8081"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("[Config] %s not found, using default", key)
	return fallback
}
