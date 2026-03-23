// Package config provides shared configuration loading for OpsNexus services.
package config

// Base contains configuration fields shared by all services.
type Base struct {
	Service     ServiceConfig  `mapstructure:"service"`
	HTTP        HTTPConfig     `mapstructure:"http"`
	Database    DatabaseConfig `mapstructure:"database"`
	Kafka       KafkaConfig    `mapstructure:"kafka"`
	Redis       RedisConfig    `mapstructure:"redis"`
	Telemetry   TelemetryConfig `mapstructure:"telemetry"`
	Log         LogConfig      `mapstructure:"log"`
}

type ServiceConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"` // dev, staging, prod
}

type HTTPConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`  // seconds
	WriteTimeout int    `mapstructure:"write_timeout"` // seconds
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"ssl_mode"`
	MaxConns int    `mapstructure:"max_conns"`
	MinConns int    `mapstructure:"min_conns"`
}

// DSN returns the PostgreSQL connection string.
func (d *DatabaseConfig) DSN() string {
	return "postgres://" + d.User + ":" + d.Password +
		"@" + d.Host + ":" + itoa(d.Port) +
		"/" + d.Name + "?sslmode=" + d.SSLMode
}

type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	ConsumerGroup string   `mapstructure:"consumer_group"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type TelemetryConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	OTLPEndpoint  string `mapstructure:"otlp_endpoint"`
	SampleRate    float64 `mapstructure:"sample_rate"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"` // debug, info, warn, error
	Format string `mapstructure:"format"` // json, text
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
