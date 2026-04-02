package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	IPRPS       int           // max requests per second for IP
	IPBlockTime time.Duration // block duration when IP limit exceeded

	TokenRPS       map[string]int           // token -> max requests per second
	TokenBlockTime map[string]time.Duration // token -> block duration when limit exceeded

	DefaultBlockTime time.Duration
}

func Load() *Config {
	cfg := &Config{
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		IPRPS:       getEnvInt("IP_MAX_REQUESTS", 10),
		IPBlockTime: getEnvDuration("IP_BLOCK_TIME", 5*time.Minute),

		TokenRPS:       make(map[string]int),
		TokenBlockTime: make(map[string]time.Duration),

		DefaultBlockTime: getEnvDuration("DEFAULT_BLOCK_TIME", 5*time.Minute),
	}

	// Load token-specific configs from env vars with pattern TOKEN_<NAME>_MAX_REQUESTS and TOKEN_<NAME>_BLOCK_TIME
	// For simplicity, also support a generic TOKEN_MAX_REQUESTS and TOKEN_BLOCK_TIME
	tokenDefaultRPS := getEnvInt("TOKEN_MAX_REQUESTS", 100)
	tokenDefaultBlock := getEnvDuration("TOKEN_BLOCK_TIME", 5*time.Minute)

	// Scan environment for token-specific overrides
	for _, env := range os.Environ() {
		key, value := splitEnv(env)
		if len(key) > 22 && key[:16] == "TOKEN_" && key[len(key)-12:] == "_MAX_REQUESTS" {
			tokenName := key[16 : len(key)-12]
			if rps, err := strconv.Atoi(value); err == nil {
				cfg.TokenRPS[tokenName] = rps
			}
		}
		if len(key) > 17 && key[:16] == "TOKEN_" && key[len(key)-10:] == "_BLOCK_TIME" {
			tokenName := key[16 : len(key)-10]
			if d, err := time.ParseDuration(value); err == nil {
				cfg.TokenBlockTime[tokenName] = d
			}
		}
	}

	// Store defaults under key "" so the limiter can use them
	cfg.TokenRPS[""] = tokenDefaultRPS
	cfg.TokenBlockTime[""] = tokenDefaultBlock

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func splitEnv(env string) (string, string) {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return env[:i], env[i+1:]
		}
	}
	return env, ""
}
