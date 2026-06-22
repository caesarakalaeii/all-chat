// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package config loads the youtube-quota-monitor runtime configuration from the
// environment (the shared allchat-config ConfigMap in production).
package config

import (
	"os"
	"time"
)

// Config holds runtime configuration.
type Config struct {
	Port             string
	GinMode          string
	LogLevel         string
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
	RedisHost        string
	RedisPort        string

	// MonitorInterval is how often the shared quota table is polled.
	MonitorInterval time.Duration
	// CleanupInterval is how often stale (past-day) quota reservations are swept.
	CleanupInterval time.Duration
	// NotifierEnabled toggles publishing QuotaEvents to the alert channel.
	NotifierEnabled bool
	// AlertChannel is the Redis Pub/Sub channel the discord-bot subscribes to.
	AlertChannel string
}

// Load reads configuration from the environment, applying defaults.
func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "8093"),
		GinMode:          getEnv("GIN_MODE", "debug"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		DatabaseHost:     getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:     getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:     getEnv("DATABASE_USER", "allchat"),
		DatabasePassword: getEnv("DATABASE_PASSWORD", "allchat_dev_password"),
		DatabaseName:     getEnv("DATABASE_NAME", "allchat"),
		RedisHost:        getEnv("REDIS_HOST", "localhost"),
		RedisPort:        getEnv("REDIS_PORT", "6379"),
		MonitorInterval:  getEnvDuration("QUOTA_MONITOR_INTERVAL", 30*time.Second),
		CleanupInterval:  getEnvDuration("QUOTA_CLEANUP_INTERVAL", 5*time.Minute),
		NotifierEnabled:  getEnv("QUOTA_NOTIFIER_ENABLED", "true") == "true",
		AlertChannel:     getEnv("QUOTA_ALERT_CHANNEL", "quota:alerts"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
