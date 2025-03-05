// Package config provides functionality for loading and accessing application configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	// Environment is the current running environment (development, staging, production)
	Environment string `mapstructure:"environment"`

	// Server configuration
	Server struct {
		// Port is the HTTP server port
		Port int `mapstructure:"port"`
		// Host is the HTTP server host
		Host string `mapstructure:"host"`
		// ReadTimeout is the maximum duration for reading the entire request
		ReadTimeout time.Duration `mapstructure:"read_timeout"`
		// WriteTimeout is the maximum duration before timing out writes of the response
		WriteTimeout time.Duration `mapstructure:"write_timeout"`
		// IdleTimeout is the maximum amount of time to wait for the next request
		IdleTimeout time.Duration `mapstructure:"idle_timeout"`
		// TrustedProxies is the list of trusted proxy IP addresses
		TrustedProxies []string `mapstructure:"trusted_proxies"`
		// UseHTTPS indicates whether to enable HTTPS
		UseHTTPS bool `mapstructure:"use_https"`
		// CertFile is the path to the TLS certificate file
		CertFile string `mapstructure:"cert_file"`
		// KeyFile is the path to the TLS key file
		KeyFile string `mapstructure:"key_file"`
	} `mapstructure:"server"`

	// Database configuration
	Database struct {
		// MongoDB configuration
		MongoDB struct {
			// URI is the MongoDB connection URI
			URI string `mapstructure:"uri"`
			// Database is the MongoDB database name
			Database string `mapstructure:"database"`
			// Timeout is the MongoDB operation timeout
			Timeout time.Duration `mapstructure:"timeout"`
			// MaxPoolSize is the maximum number of connections in the connection pool
			MaxPoolSize uint64 `mapstructure:"max_pool_size"`
			// MinPoolSize is the minimum number of connections in the connection pool
			MinPoolSize uint64 `mapstructure:"min_pool_size"`
			// MaxIdleTime is the maximum idle time for a connection
			MaxIdleTime time.Duration `mapstructure:"max_idle_time"`
		} `mapstructure:"mongodb"`

		// Redis configuration
		Redis struct {
			// Addresses is the list of Redis server addresses
			Addresses []string `mapstructure:"addresses"`
			// Username is the Redis username
			Username string `mapstructure:"username"`
			// Password is the Redis password
			Password string `mapstructure:"password"`
			// Database is the Redis database index
			Database int `mapstructure:"database"`
			// MaxRetries is the maximum number of retries for Redis operations
			MaxRetries int `mapstructure:"max_retries"`
			// PoolSize is the Redis connection pool size
			PoolSize int `mapstructure:"pool_size"`
			// MinIdleConns is the minimum number of idle connections
			MinIdleConns int `mapstructure:"min_idle_conns"`
			// DialTimeout is the timeout for establishing new connections
			DialTimeout time.Duration `mapstructure:"dial_timeout"`
			// ReadTimeout is the timeout for Redis reads
			ReadTimeout time.Duration `mapstructure:"read_timeout"`
			// WriteTimeout is the timeout for Redis writes
			WriteTimeout time.Duration `mapstructure:"write_timeout"`
			// IdleTimeout is the timeout for idle connections
			IdleTimeout time.Duration `mapstructure:"idle_timeout"`
		} `mapstructure:"redis"`
	} `mapstructure:"database"`

	// Authentication configuration
	Auth struct {
		// JWTSecret is the secret key for signing JWTs
		JWTSecret string `mapstructure:"jwt_secret"`
		// AccessTokenExpiry is the expiry time for access tokens
		AccessTokenExpiry time.Duration `mapstructure:"access_token_expiry"`
		// RefreshTokenExpiry is the expiry time for refresh tokens
		RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
		// PasswordMinLength is the minimum password length
		PasswordMinLength int `mapstructure:"password_min_length"`
		// PasswordMaxLength is the maximum password length
		PasswordMaxLength int `mapstructure:"password_max_length"`
		// PasswordResetExpiry is the expiry time for password reset tokens
		PasswordResetExpiry time.Duration `mapstructure:"password_reset_expiry"`
		// AllowedOrigins is the list of allowed CORS origins
		AllowedOrigins []string `mapstructure:"allowed_origins"`
	} `mapstructure:"auth"`

	// Media configuration
	Media struct {
		// YouTube API key
		YouTubeAPIKey string `mapstructure:"youtube_api_key"`
		// SoundCloud API key
		SoundCloudAPIKey string `mapstructure:"soundcloud_api_key"`
		// AllowedSources is the list of allowed media sources
		AllowedSources []string `mapstructure:"allowed_sources"`
		// MaxDuration is the maximum allowed media duration in seconds
		MaxDuration int `mapstructure:"max_duration"`
		// CacheExpiry is the expiry time for media cache
		CacheExpiry time.Duration `mapstructure:"cache_expiry"`
	} `mapstructure:"media"`

	// Room configuration
	Room struct {
		// MaxRooms is the maximum number of active rooms
		MaxRooms int `mapstructure:"max_rooms"`
		// MaxUsersPerRoom is the maximum number of users per room
		MaxUsersPerRoom int `mapstructure:"max_users_per_room"`
		// MaxDJQueueSize is the maximum size of the DJ queue
		MaxDJQueueSize int `mapstructure:"max_dj_queue_size"`
		// RoomInactiveTimeout is the time after which an inactive room is closed
		RoomInactiveTimeout time.Duration `mapstructure:"room_inactive_timeout"`
		// DefaultRoomTheme is the default room theme
		DefaultRoomTheme string `mapstructure:"default_room_theme"`
		// AvailableThemes is the list of available room themes
		AvailableThemes []string `mapstructure:"available_themes"`
	} `mapstructure:"room"`

	// WebSocket configuration
	WebSocket struct {
		// MaxMessageSize is the maximum message size
		MaxMessageSize int64 `mapstructure:"max_message_size"`
		// WriteWait is the time allowed to write a message to the peer
		WriteWait time.Duration `mapstructure:"write_wait"`
		// PongWait is the time allowed to read the next pong message from the peer
		PongWait time.Duration `mapstructure:"pong_wait"`
		// PingPeriod is the time between ping messages
		PingPeriod time.Duration `mapstructure:"ping_period"`
		// MaxConnections is the maximum number of concurrent WebSocket connections
		MaxConnections int `mapstructure:"max_connections"`
	} `mapstructure:"websocket"`

	// Logging configuration
	Logging struct {
		// Level is the logging level
		Level string `mapstructure:"level"`
		// Format is the logging format (json or console)
		Format string `mapstructure:"format"`
		// OutputPaths is the list of output paths for logs
		OutputPaths []string `mapstructure:"output_paths"`
		// ErrorOutputPaths is the list of output paths for error logs
		ErrorOutputPaths []string `mapstructure:"error_output_paths"`
	} `mapstructure:"logging"`

	// Feature flags
	Features struct {
		// EnableRegistration determines whether new user registration is enabled
		EnableRegistration bool `mapstructure:"enable_registration"`
		// EnableRoomCreation determines whether room creation is enabled
		EnableRoomCreation bool `mapstructure:"enable_room_creation"`
		// EnableChatCommands determines whether chat commands are enabled
		EnableChatCommands bool `mapstructure:"enable_chat_commands"`
		// EnableAvatars determines whether avatars are enabled
		EnableAvatars bool `mapstructure:"enable_avatars"`
		// EnableSoundCloud determines whether SoundCloud integration is enabled
		EnableSoundCloud bool `mapstructure:"enable_soundcloud"`
		// EnableProfanityFilter determines whether profanity filter is enabled
		EnableProfanityFilter bool `mapstructure:"enable_profanity_filter"`
	} `mapstructure:"features"`
}

// LoadConfig loads the configuration from file and environment variables.
// It looks for a configuration file in the following locations:
// 1. Path specified in the CONFIG_FILE environment variable
// 2. ./configs directory
// 3. ../configs directory
// 4. /etc/musicroom directory
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Configuration file name and type
	v.SetConfigName("app")
	v.SetConfigType("yaml")

	// Add configuration paths
	configFile := os.Getenv("CONFIG_FILE")
	if configFile != "" {
		// Use configuration file from environment variable
		v.SetConfigFile(configFile)
	} else {
		// Search for configuration in common directories
		v.AddConfigPath("./configs")
		v.AddConfigPath("../configs")
		v.AddConfigPath("/etc/musicroom")
	}

	// Read the configuration file
	if err := v.ReadInConfig(); err != nil {
		// If the configuration file is not found, use environment variables and defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Check for environment-specific configuration file
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development" // Default environment
	}

	v.SetConfigName(fmt.Sprintf("app.%s", env))
	// Try to merge the environment-specific configuration file
	if err := v.MergeInConfig(); err != nil {
		// Ignore file not found error for environment config
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to merge environment config file: %w", err)
		}
	}

	// Override with environment variables
	v.SetEnvPrefix("APP") // Prefix for environment variables
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Load the configuration into the Config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set the environment
	config.Environment = env

	// Validate the configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets the default values for the configuration
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.use_https", false)

	// Database defaults
	v.SetDefault("database.mongodb.uri", "mongodb://localhost:27017")
	v.SetDefault("database.mongodb.database", "musicroom")
	v.SetDefault("database.mongodb.timeout", "10s")
	v.SetDefault("database.mongodb.max_pool_size", 100)
	v.SetDefault("database.mongodb.min_pool_size", 10)
	v.SetDefault("database.mongodb.max_idle_time", "60s")

	v.SetDefault("database.redis.addresses", []string{"localhost:6379"})
	v.SetDefault("database.redis.database", 0)
	v.SetDefault("database.redis.max_retries", 3)
	v.SetDefault("database.redis.pool_size", 100)
	v.SetDefault("database.redis.min_idle_conns", 10)
	v.SetDefault("database.redis.dial_timeout", "5s")
	v.SetDefault("database.redis.read_timeout", "3s")
	v.SetDefault("database.redis.write_timeout", "3s")
	v.SetDefault("database.redis.idle_timeout", "300s")

	// Authentication defaults
	v.SetDefault("auth.access_token_expiry", "15m")
	v.SetDefault("auth.refresh_token_expiry", "168h") // 7 days
	v.SetDefault("auth.password_min_length", 8)
	v.SetDefault("auth.password_max_length", 72)
	v.SetDefault("auth.password_reset_expiry", "1h")
	v.SetDefault("auth.allowed_origins", []string{"*"})

	// Media defaults
	v.SetDefault("media.allowed_sources", []string{"youtube", "soundcloud"})
	v.SetDefault("media.max_duration", 600) // 10 minutes
	v.SetDefault("media.cache_expiry", "24h")

	// Room defaults
	v.SetDefault("room.max_rooms", 100)
	v.SetDefault("room.max_users_per_room", 200)
	v.SetDefault("room.max_dj_queue_size", 50)
	v.SetDefault("room.room_inactive_timeout", "6h")
	v.SetDefault("room.default_room_theme", "default")
	v.SetDefault("room.available_themes", []string{"default", "dark", "light", "neon", "vintage"})

	// WebSocket defaults
	v.SetDefault("websocket.max_message_size", 4096)
	v.SetDefault("websocket.write_wait", "10s")
	v.SetDefault("websocket.pong_wait", "60s")
	v.SetDefault("websocket.ping_period", "54s")
	v.SetDefault("websocket.max_connections", 10000)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output_paths", []string{"stdout"})
	v.SetDefault("logging.error_output_paths", []string{"stderr"})

	// Feature flags defaults
	v.SetDefault("features.enable_registration", true)
	v.SetDefault("features.enable_room_creation", true)
	v.SetDefault("features.enable_chat_commands", true)
	v.SetDefault("features.enable_avatars", true)
	v.SetDefault("features.enable_soundcloud", true)
	v.SetDefault("features.enable_profanity_filter", true)
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return errors.New("server port must be between 1 and 65535")
	}

	// Validate JWT Secret
	if config.Auth.JWTSecret == "" {
		return errors.New("JWT secret must be set")
	}

	// Check if HTTPS is enabled but certificates are not configured
	if config.Server.UseHTTPS {
		if config.Server.CertFile == "" || config.Server.KeyFile == "" {
			return errors.New("TLS certificate and key files must be provided when HTTPS is enabled")
		}

		// Check if certificate and key files exist
		if _, err := os.Stat(config.Server.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS certificate file not found: %s", config.Server.CertFile)
		}

		if _, err := os.Stat(config.Server.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file not found: %s", config.Server.KeyFile)
		}
	}

	// Validate MongoDB configuration
	if config.Database.MongoDB.URI == "" {
		return errors.New("MongoDB URI must be set")
	}

	// Validate Redis configuration
	if len(config.Database.Redis.Addresses) == 0 {
		return errors.New("at least one Redis address must be provided")
	}

	// Validate media configuration
	if config.Features.EnableSoundCloud && config.Media.SoundCloudAPIKey == "" {
		return errors.New("SoundCloud API key must be set when SoundCloud integration is enabled")
	}

	if len(config.Media.AllowedSources) == 0 {
		return errors.New("at least one allowed media source must be provided")
	}

	return nil
}

// GetConfigString returns a formatted string with the current configuration
func GetConfigString(config *Config) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Environment: %s\n", config.Environment))
	sb.WriteString(fmt.Sprintf("Server: %s:%d\n", config.Server.Host, config.Server.Port))
	sb.WriteString(fmt.Sprintf("MongoDB Database: %s\n", config.Database.MongoDB.Database))
	sb.WriteString(fmt.Sprintf("Redis Database: %d\n", config.Database.Redis.Database))
	sb.WriteString(fmt.Sprintf("Max Rooms: %d\n", config.Room.MaxRooms))
	sb.WriteString(fmt.Sprintf("Max Users Per Room: %d\n", config.Room.MaxUsersPerRoom))
	sb.WriteString(fmt.Sprintf("Allowed Media Sources: %s\n", strings.Join(config.Media.AllowedSources, ", ")))
	sb.WriteString(fmt.Sprintf("Max Media Duration: %d seconds\n", config.Media.MaxDuration))
	sb.WriteString("Features:\n")
	sb.WriteString(fmt.Sprintf("  Registration Enabled: %t\n", config.Features.EnableRegistration))
	sb.WriteString(fmt.Sprintf("  Room Creation Enabled: %t\n", config.Features.EnableRoomCreation))
	sb.WriteString(fmt.Sprintf("  SoundCloud Enabled: %t\n", config.Features.EnableSoundCloud))
	sb.WriteString(fmt.Sprintf("  Avatars Enabled: %t\n", config.Features.EnableAvatars))

	return sb.String()
}

// EnsureConfigDirs ensures that all necessary directories for configuration exist
func EnsureConfigDirs() error {
	dirs := []string{
		"./configs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// WriteDefaultConfig writes the default configuration files
func WriteDefaultConfig() error {
	if err := EnsureConfigDirs(); err != nil {
		return err
	}

	// Create default configuration file
	defaultConfigPath := filepath.Join("./configs", "app.yaml")
	if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
		defaultConfig := `# Music Room Application Configuration

# Server configuration
server:
  port: 8080
  host: "0.0.0.0"
  read_timeout: "15s"
  write_timeout: "15s"
  idle_timeout: "60s"
  use_https: false
  cert_file: ""
  key_file: ""
  trusted_proxies: []

# Database configuration
database:
  mongodb:
    uri: "mongodb://localhost:27017"
    database: "musicroom"
    timeout: "10s"
    max_pool_size: 100
    min_pool_size: 10
    max_idle_time: "60s"
  redis:
    addresses: ["localhost:6379"]
    password: ""
    database: 0
    max_retries: 3
    pool_size: 100
    min_idle_conns: 10
    dial_timeout: "5s"
    read_timeout: "3s"
    write_timeout: "3s"
    idle_timeout: "300s"

# Authentication configuration
auth:
  jwt_secret: "" # Must be set in environment or secrets file
  access_token_expiry: "15m"
  refresh_token_expiry: "168h" # 7 days
  password_min_length: 8
  password_max_length: 72
  password_reset_expiry: "1h"
  allowed_origins: ["*"]

# Media configuration
media:
  youtube_api_key: "" # Must be set in environment or secrets file
  soundcloud_api_key: "" # Must be set in environment or secrets file
  allowed_sources: ["youtube", "soundcloud"]
  max_duration: 600 # 10 minutes
  cache_expiry: "24h"

# Room configuration
room:
  max_rooms: 100
  max_users_per_room: 200
  max_dj_queue_size: 50
  room_inactive_timeout: "6h"
  default_room_theme: "default"
  available_themes: ["default", "dark", "light", "neon", "vintage"]

# WebSocket configuration
websocket:
  max_message_size: 4096
  write_wait: "10s"
  pong_wait: "60s"
  ping_period: "54s"
  max_connections: 10000

# Logging configuration
logging:
  level: "info"
  format: "json"
  output_paths: ["stdout"]
  error_output_paths: ["stderr"]

# Feature flags
features:
  enable_registration: true
  enable_room_creation: true
  enable_chat_commands: true
  enable_avatars: true
  enable_soundcloud: true
  enable_profanity_filter: true
`
		if err := os.WriteFile(defaultConfigPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed to write default config file: %w", err)
		}
	}

	// Create development configuration file
	devConfigPath := filepath.Join("./configs", "app.development.yaml")
	if _, err := os.Stat(devConfigPath); os.IsNotExist(err) {
		devConfig := `# Development environment configuration
# This file overrides the values in app.yaml for the development environment

# Server configuration
server:
  port: 8080
  host: "localhost"

# Logging configuration
logging:
  level: "debug"
  format: "console"

# Feature flags for development
features:
  enable_registration: true
  enable_room_creation: true
  enable_profanity_filter: false # Disabled for development
`
		if err := os.WriteFile(devConfigPath, []byte(devConfig), 0644); err != nil {
			return fmt.Errorf("failed to write development config file: %w", err)
		}
	}

	// Create production configuration file
	prodConfigPath := filepath.Join("./configs", "app.production.yaml")
	if _, err := os.Stat(prodConfigPath); os.IsNotExist(err) {
		prodConfig := `# Production environment configuration
# This file overrides the values in app.yaml for the production environment

# Server configuration
server:
  use_https: true
  cert_file: "/etc/ssl/certs/mycert.pem"
  key_file: "/etc/ssl/private/mykey.pem"
  trusted_proxies: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]

# Logging configuration
logging:
  level: "info"
  format: "json"
  output_paths: ["stdout", "/var/log/musicroom/app.log"]
  error_output_paths: ["stderr", "/var/log/musicroom/error.log"]

# More restrictive room configuration for production
room:
  max_rooms: 500
  max_users_per_room: 500
  room_inactive_timeout: "12h"

# Feature flags for production
features:
  enable_registration: true
  enable_room_creation: true
  enable_profanity_filter: true
`
		if err := os.WriteFile(prodConfigPath, []byte(prodConfig), 0644); err != nil {
			return fmt.Errorf("failed to write production config file: %w", err)
		}
	}

	// Create secrets example file
	secretsExamplePath := filepath.Join("./configs", "secrets.example.yaml")
	if _, err := os.Stat(secretsExamplePath); os.IsNotExist(err) {
		secretsExample := `# Secrets configuration
# Copy this file to secrets.yaml and fill in the values

# Authentication configuration
auth:
  jwt_secret: "replace_with_a_secure_random_string"

# Media configuration
media:
  youtube_api_key: "your_youtube_api_key"
  soundcloud_api_key: "your_soundcloud_api_key"

# Database configuration
database:
  mongodb:
    uri: "mongodb://username:password@localhost:27017"
  redis:
    password: "your_redis_password"
`
		if err := os.WriteFile(secretsExamplePath, []byte(secretsExample), 0644); err != nil {
			return fmt.Errorf("failed to write secrets example file: %w", err)
		}
	}

	return nil
}
