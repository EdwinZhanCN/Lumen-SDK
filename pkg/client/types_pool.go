package client

import "time"

type ConnectionStatus string

const (
	ConnectionStatusConnected    ConnectionStatus = "connected"
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
	ConnectionStatusError        ConnectionStatus = "error"
)

type PoolConfig struct {
	// 连接池管理参数
	MaxConnections int           `yaml:"max_connections" json:"max_connections"`
	MaxIdleTime    time.Duration `yaml:"max_idle_time" json:"max_idle_time"`
	MaxLifetime    time.Duration `yaml:"max_lifetime" json:"max_lifetime"`
	ConnectionTTL  time.Duration `yaml:"connection_ttl" json:"connection_ttl"`
	HealthCheck    bool          `yaml:"health_check" json:"health_check"`
	HealthInterval time.Duration `yaml:"health_interval" json:"health_interval"`
}
