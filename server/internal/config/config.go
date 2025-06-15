package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Env     string       `json:"env"`
	NodeEnv string       `json:"nodeEnv"`
	UI      UIConfig     `json:"ui"`
	Server  ServerConfig `json:"server"`
	Nginx   NginxConfig  `json:"nginx"`
}

type UIConfig struct {
	Port   int    `json:"port"`
	WsUrl  string `json:"wsUrl"`
	ApiUrl string `json:"apiUrl"`
}

type ServerConfig struct {
	Port           int      `json:"port"`
	DbPath         string   `json:"dbPath"`
	AllowedOrigins []string `json:"allowedOrigins"`
}

type NginxConfig struct {
	HttpPort  int `json:"httpPort"`
	HttpsPort int `json:"httpsPort"`
}

var config Config

func Load() error {
	env := os.Getenv("ENV")
	if env == "" {
		env = "prod" // Default to production
	}

	// Map environment values to config file names
	configFile := "prod.json"
	if env == "development" {
		configFile = "dev.json"
	}

	// Get the project root directory
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		// Default to current directory if not set
		projectRoot = "."
	}

	// Read the config file
	configPath := filepath.Join(projectRoot, "config", configFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Parse the JSON config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	return nil
}

func Get() *Config {
	return &config
}
