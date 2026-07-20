package config

import "time"

type Config struct {
	DataDir       string
	MergeInterval time.Duration
	FsyncStrategy string        // "always", "interval", "none"
	FsyncInterval time.Duration // e.g. 1 * time.Second
}

func Load() (*Config, error) {
	// TODO: Load configuration from yaml files or environment variables
	return &Config{
		DataDir:       "data",
		MergeInterval: 30 * time.Minute,
		FsyncStrategy: "none",
		FsyncInterval: 1 * time.Second,
	}, nil
}
