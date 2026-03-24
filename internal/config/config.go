package config

import (
	"os"
	"strconv"
	"strings"
)

type GatewaySeed struct {
	Port     int
	Provider string
}

type Config struct {
	AdminPort       int
	LlamaCppURL     string
	OnlineBaseURL   string
	OnlineAPIKey    string
	OnlineModel     string
	DefaultGateways []GatewaySeed
}

func LoadFromEnv() Config {
	cfg := Config{
		AdminPort:     envInt("ADMIN_PORT", 8080),
		LlamaCppURL:   os.Getenv("LLAMA_CPP_URL"),
		OnlineBaseURL: os.Getenv("ONLINE_BASE_URL"),
		OnlineAPIKey:  os.Getenv("ONLINE_API_KEY"),
		OnlineModel:   envString("ONLINE_MODEL", "gpt-4o-mini"),
	}

	if seeds := os.Getenv("DEFAULT_GATEWAYS"); seeds != "" {
		for _, part := range strings.Split(seeds, ",") {
			items := strings.Split(strings.TrimSpace(part), ":")
			if len(items) != 2 {
				continue
			}
			port, err := strconv.Atoi(items[0])
			if err != nil {
				continue
			}
			cfg.DefaultGateways = append(cfg.DefaultGateways, GatewaySeed{Port: port, Provider: items[1]})
		}
	}

	return cfg
}

func envInt(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envString(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
