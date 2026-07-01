package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const maxConfigSize = 1 << 20 // 1MB

type Config struct {
	ClusterName string  `json:"cluster_name"`
	Interval    string  `json:"interval"`
	CPUPerCore  float64 `json:"cpu_per_core_hour"`
	MemPerGB    float64 `json:"mem_per_gb_hour"`
	WasteRatio  float64 `json:"waste_ratio"`
	MinWaste    float64 `json:"min_waste_dollars"`
	Port        int     `json:"port"`
}

func Default() *Config {
	return &Config{
		ClusterName: "unknown",
		Interval:    "15m",
		CPUPerCore:  0.0316,
		MemPerGB:    0.0042,
		WasteRatio:  1.5,
		MinWaste:    5.00,
		Port:        8080,
	}
}

// Load reads and validates a JSON config file.
// Missing file is not an error — returns defaults.
func Load(path string) (*Config, error) {
	cfg := Default()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	data := make([]byte, maxConfigSize+1)
	n, err := f.Read(data)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if n > maxConfigSize {
		return nil, fmt.Errorf("config > 1MB")
	}
	if err := json.Unmarshal(data[:n], cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func (c *Config) ParseInterval() (time.Duration, error) {
	return time.ParseDuration(c.Interval)
}

func (c *Config) validate() error {
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return fmt.Errorf("interval %q: %w", c.Interval, err)
	}
	if d < time.Second {
		return fmt.Errorf("interval %s < 1s", d)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port %d out of range", c.Port)
	}
	if c.CPUPerCore <= 0 {
		return fmt.Errorf("cpu_per_core_hour must be > 0")
	}
	if c.MemPerGB <= 0 {
		return fmt.Errorf("mem_per_gb_hour must be > 0")
	}
	if c.WasteRatio <= 1.0 {
		return fmt.Errorf("waste_ratio must be > 1.0")
	}
	if c.MinWaste < 0 {
		return fmt.Errorf("min_waste_dollars must be >= 0")
	}
	return nil
}
