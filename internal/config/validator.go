// Package config provides functionality for loading and accessing application configuration.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ValidateAndFixConfig validates the configuration and fixes any issues
func ValidateAndFixConfig(config *Config) []string {
	var warnings []string

	// Check JWT secret
	if config.Auth.JWTSecret == "" {
		warnings = append(warnings, "JWT secret is not set, generating a random one")
		secret, err := generateRandomSecret(32)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to generate JWT secret: %v", err))
		} else {
			config.Auth.JWTSecret = secret
		}
	} else if len(config.Auth.JWTSecret) < 16 {
		warnings = append(warnings, "JWT secret is too short, should be at least 16 characters")
	}

	// Check server timeouts
	minTimeout := 1 * time.Second
	maxTimeout := 5 * time.Minute

	if config.Server.ReadTimeout < minTimeout {
		warnings = append(warnings, fmt.Sprintf("Server read timeout is too short (%v), setting to %v", config.Server.ReadTimeout, minTimeout))
		config.Server.ReadTimeout = minTimeout
	} else if config.Server.ReadTimeout > maxTimeout {
		warnings = append(warnings, fmt.Sprintf("Server read timeout is too long (%v), setting to %v", config.Server.ReadTimeout, maxTimeout))
		config.Server.ReadTimeout = maxTimeout
	}

	if config.Server.WriteTimeout < minTimeout {
		warnings = append(warnings, fmt.Sprintf("Server write timeout is too short (%v), setting to %v", config.Server.WriteTimeout, minTimeout))
		config.Server.WriteTimeout = minTimeout
	} else if config.Server.WriteTimeout > maxTimeout {
		warnings = append(warnings, fmt.Sprintf("Server write timeout is too long (%v), setting to %v", config.Server.WriteTimeout, maxTimeout))
		config.Server.WriteTimeout = maxTimeout
	}

	if config.Server.IdleTimeout < minTimeout {
		warnings = append(warnings, fmt.Sprintf("Server idle timeout is too short (%v), setting to %v", config.Server.IdleTimeout, minTimeout))
		config.Server.IdleTimeout = minTimeout
	}

	// Check MongoDB connection string
	if !strings.HasPrefix(config.Database.MongoDB.URI, "mongodb://") && !strings.HasPrefix(config.Database.MongoDB.URI, "mongodb+srv://") {
		warnings = append(warnings, "MongoDB URI is invalid, must start with mongodb:// or mongodb+srv://")
	}

	// Check Redis addresses
	for _, addr := range config.Database.Redis.Addresses {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Invalid Redis address: %s", addr))
			continue
		}

		if host == "" {
			warnings = append(warnings, fmt.Sprintf("Redis address has empty host: %s", addr))
		}

		if port == "" {
			warnings = append(warnings, fmt.Sprintf("Redis address has empty port: %s", addr))
		}
	}

	// Check media configuration
	if len(config.Media.AllowedSources) == 0 {
		warnings = append(warnings, "No media sources are allowed, adding 'youtube' as default")
		config.Media.AllowedSources = []string{"youtube"}
	}

	// Check if YouTube is enabled but API key is missing
	youtubeEnabled := false
	for _, source := range config.Media.AllowedSources {
		if strings.ToLower(source) == "youtube" {
			youtubeEnabled = true
			break
		}
	}

	if youtubeEnabled && config.Media.YouTubeAPIKey == "" {
		warnings = append(warnings, "YouTube is enabled but API key is not set")
	}

	// Check if SoundCloud is enabled but API key is missing
	soundcloudEnabled := false
	for _, source := range config.Media.AllowedSources {
		if strings.ToLower(source) == "soundcloud" {
			soundcloudEnabled = true
			break
		}
	}

	if soundcloudEnabled && config.Media.SoundCloudAPIKey == "" {
		warnings = append(warnings, "SoundCloud is enabled but API key is not set")
	}

	// Check logging configuration
	validLevels := map[string]bool{
		"debug":  true,
		"info":   true,
		"warn":   true,
		"error":  true,
		"dpanic": true,
		"panic":  true,
		"fatal":  true,
	}

	if !validLevels[strings.ToLower(config.Logging.Level)] {
		warnings = append(warnings, fmt.Sprintf("Invalid logging level: %s, setting to 'info'", config.Logging.Level))
		config.Logging.Level = "info"
	}

	validFormats := map[string]bool{
		"json":    true,
		"console": true,
	}

	if !validFormats[strings.ToLower(config.Logging.Format)] {
		warnings = append(warnings, fmt.Sprintf("Invalid logging format: %s, setting to 'json'", config.Logging.Format))
		config.Logging.Format = "json"
	}

	// Check if output paths exist and are writable
	for _, path := range config.Logging.OutputPaths {
		if path != "stdout" && path != "stderr" {
			dir := filepath.Dir(path)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("Log output directory does not exist: %s", dir))
			} else {
				// Check if directory is writable
				testFile := filepath.Join(dir, ".test_write")
				if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
					warnings = append(warnings, fmt.Sprintf("Log output directory is not writable: %s", dir))
				} else {
					os.Remove(testFile)
				}
			}
		}
	}

	return warnings
}

// generateRandomSecret generates a random secret string of the specified length
func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// GetLogLevel converts a string log level to a zap log level
func GetLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// IsFeatureEnabled checks if a feature is enabled in the configuration
func IsFeatureEnabled(config *Config, feature string) bool {
	val := reflect.ValueOf(config.Features)
	field := val.FieldByName(feature)

	if !field.IsValid() || field.Kind() != reflect.Bool {
		return false
	}

	return field.Bool()
}

// ConfigureLogger configures the logger based on the configuration
func ConfigureLogger(config *Config) (*zap.Logger, error) {
	level := GetLogLevel(config.Logging.Level)

	// Configure logger
	logConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: config.Environment == "development",
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         config.Logging.Format,
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      config.Logging.OutputPaths,
		ErrorOutputPaths: config.Logging.ErrorOutputPaths,
	}

	// Customize encoder for console format
	if config.Logging.Format == "console" {
		logConfig.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	// Build the logger
	return logConfig.Build()
}

// CreateDefaultConfig creates the default configuration
func CreateDefaultConfig() *Config {
	config := &Config{}

	// Set default environment
	config.Environment = "development"

	// Set default server configuration
	config.Server.Port = 8080
	config.Server.Host = "0.0.0.0"
	config.Server.ReadTimeout = 15 * time.Second
	config.Server.WriteTimeout = 15 * time.Second
	config.Server.IdleTimeout = 60 * time.Second
	config.Server.UseHTTPS = false

	// Set default MongoDB configuration
	config.Database.MongoDB.URI = "mongodb://localhost:27017"
	config.Database.MongoDB.Database = "musicroom"
	config.Database.MongoDB.Timeout = 10 * time.Second
	config.Database.MongoDB.MaxPoolSize = 100
	config.Database.MongoDB.MinPoolSize = 10
	config.Database.MongoDB.MaxIdleTime = 60 * time.Second

	// Set default Redis configuration
	config.Database.Redis.Addresses = []string{"localhost:6379"}
	config.Database.Redis.Database = 0
	config.Database.Redis.MaxRetries = 3
	config.Database.Redis.PoolSize = 100
	config.Database.Redis.MinIdleConns = 10
	config.Database.Redis.DialTimeout = 5 * time.Second
	config.Database.Redis.ReadTimeout = 3 * time.Second
	config.Database.Redis.WriteTimeout = 3 * time.Second
	config.Database.Redis.IdleTimeout = 300 * time.Second

	// Set default authentication configuration
	secret, err := generateRandomSecret(32)
	if err == nil {
		config.Auth.JWTSecret = secret
	}
	config.Auth.AccessTokenExpiry = 15 * time.Minute
	config.Auth.RefreshTokenExpiry = 7 * 24 * time.Hour
	config.Auth.PasswordMinLength = 8
	config.Auth.PasswordMaxLength = 72
	config.Auth.PasswordResetExpiry = 1 * time.Hour
	config.Auth.AllowedOrigins = []string{"*"}

	// Set default media configuration
	config.Media.AllowedSources = []string{"youtube", "soundcloud"}
	config.Media.MaxDuration = 600 // 10 minutes
	config.Media.CacheExpiry = 24 * time.Hour

	// Set default room configuration
	config.Room.MaxRooms = 100
	config.Room.MaxUsersPerRoom = 200
	config.Room.MaxDJQueueSize = 50
	config.Room.RoomInactiveTimeout = 6 * time.Hour
	config.Room.DefaultRoomTheme = "default"
	config.Room.AvailableThemes = []string{"default", "dark", "light", "neon", "vintage"}

	// Set default WebSocket configuration
	config.WebSocket.MaxMessageSize = 4096
	config.WebSocket.WriteWait = 10 * time.Second
	config.WebSocket.PongWait = 60 * time.Second
	config.WebSocket.PingPeriod = 54 * time.Second
	config.WebSocket.MaxConnections = 10000

	// Set default logging configuration
	config.Logging.Level = "info"
	config.Logging.Format = "json"
	config.Logging.OutputPaths = []string{"stdout"}
	config.Logging.ErrorOutputPaths = []string{"stderr"}

	// Set default feature flags
	config.Features.EnableRegistration = true
	config.Features.EnableRoomCreation = true
	config.Features.EnableChatCommands = true
	config.Features.EnableAvatars = true
	config.Features.EnableSoundCloud = true
	config.Features.EnableProfanityFilter = true

	return config
}
