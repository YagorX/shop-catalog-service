package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	ServiceName     string         `yaml:"service_name" env-default:"catalog-service"`
	Env             string         `yaml:"env" env-default:"local"`
	Version         string         `yaml:"version" env-default:"dev"`
	LogLevel        string         `yaml:"log_level" env-default:"info"`
	ShutdownTimeout time.Duration  `yaml:"shutdown_timeout" env-default:"10s"`
	GRPC            GRPCConfig     `yaml:"grpc"`
	HTTP            HTTPConfig     `yaml:"http"`
	OTLP            OTLPConfig     `yaml:"otlp"`
	Postgres        PostgresConfig `yaml:"postgres"`
	Redis           RedisConfig    `yaml:"redis"`
}

type GRPCConfig struct {
	Port    int           `yaml:"port" env-default:"9091"`
	Timeout time.Duration `yaml:"timeout" env-default:"5s"`
}

type HTTPConfig struct {
	Port    int           `yaml:"port" env-default:"8081"`
	Timeout time.Duration `yaml:"timeout" env-default:"5s"`
}

type OTLPConfig struct {
	Endpoint string `yaml:"endpoint" env:"OTLP_ENDPOINT" env-default:"jaeger:4317"`
}

type PostgresConfig struct {
	Host     string `yaml:"host" env:"POSTGRES_HOST" env-default:"localhost"`
	Port     int    `yaml:"port" env:"POSTGRES_PORT" env-default:"5432"`
	DBName   string `yaml:"db_name" env:"POSTGRES_DB" env-default:"catalog"`
	User     string `yaml:"user" env:"POSTGRES_USER" env-default:"catalog"`
	Password string `yaml:"password" env:"POSTGRES_PASSWORD" env-default:"catalog"`
	SSLMode  string `yaml:"ssl_mode" env:"POSTGRES_SSL_MODE" env-default:"disable"`
}

type RedisConfig struct {
	Host     string        `yaml:"host" env:"REDIS_HOST" env-default:"localhost"`
	Port     int           `yaml:"port" env:"REDIS_PORT" env-default:"6379"`
	Password string        `yaml:"password" env:"REDIS_PASSWORD" env-default:""`
	DB       int           `yaml:"db" env:"REDIS_DB" env-default:"0"`
	TTL      time.Duration `yaml:"ttl" env:"REDIS_TTL" env-default:"5m"`
}

// MustLoad is a bootstrap helper for main().
// It reads config path only from CONFIG_PATH env.
func MustLoad() *Config {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		panic("CONFIG_PATH is empty")
	}

	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}

	return cfg
}

// Load reads config from YAML and validates the result.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", path)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// MustLoadByPath keeps bootstrap code short in main().
func MustLoadByPath(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}

	return cfg
}

func (c *Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service_name is required")
	}
	if c.Env == "" {
		return fmt.Errorf("env is required")
	}
	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be in range 1..65535")
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port is required")
	}
	if c.GRPC.Timeout <= 0 {
		return fmt.Errorf("grpc.timeout must be > 0")
	}
	if c.HTTP.Timeout <= 0 {
		return fmt.Errorf("http.timeout must be > 0")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown_timeout must be > 0")
	}
	if c.OTLP.Endpoint == "" {
		return fmt.Errorf("otlp.endpoint is required")
	}

	return nil
}

func (c Config) GRPCAddr() string {
	return fmt.Sprintf(":%d", c.GRPC.Port)
}

func (c Config) HTTPAddr() string {
	return fmt.Sprintf(":%d", c.HTTP.Port)
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Postgres.User,
		c.Postgres.Password,
		c.Postgres.Host,
		c.Postgres.Port,
		c.Postgres.DBName,
		c.Postgres.SSLMode,
	)
}

func (c Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}
