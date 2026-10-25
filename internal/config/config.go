// Package config parses the initial portable TOML configuration from spec §36.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

const maxConfigBytes = 1 << 20

type Config struct {
	Storage struct {
		DataDir  string `toml:"data_dir"`
		StateDir string `toml:"state_dir"`
	} `toml:"storage"`
	Server struct {
		LoopbackAddress string `toml:"loopback_address"`
		Port            int    `toml:"port"`
	} `toml:"server"`
	LAN struct {
		Enabled         bool     `toml:"enabled"`
		ListenAddresses []string `toml:"listen_addresses"`
		AllowNetworks   []string `toml:"allow_networks"`
	} `toml:"lan"`
	Remote struct {
		Enabled        bool     `toml:"enabled"`
		PublicOrigin   string   `toml:"public_origin"`
		DirectPath     string   `toml:"direct_path"`
		TrustedProxies []string `toml:"trusted_proxies"`
	} `toml:"remote"`
	Refresh struct {
		Interval            string `toml:"interval"`
		GlobalConcurrency   int    `toml:"global_concurrency"`
		ProviderConcurrency int    `toml:"provider_concurrency"`
	} `toml:"refresh"`
	Providers struct {
		Enabled []string `toml:"enabled"`
	} `toml:"providers"`
	Relay struct {
		Enabled bool `toml:"enabled"`
	} `toml:"relay"`
}

func defaults() Config {
	var c Config
	c.Storage.DataDir = "./data"
	c.Storage.StateDir = "./state"
	c.Server.LoopbackAddress = "127.0.0.1"
	c.Server.Port = 8787
	c.Remote.DirectPath = "/direct"
	c.Remote.TrustedProxies = []string{"127.0.0.1/32"}
	c.Refresh.Interval = "6h"
	c.Refresh.GlobalConcurrency = 8
	c.Refresh.ProviderConcurrency = 3
	return c
}

func Load(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		// Do not echo paths: an operator may accidentally pass a secret as a flag.
		return Config{}, errors.New("cannot open configuration file")
	}
	defer f.Close()
	return Parse(f)
}

// Parse rejects unknown fields and caps input size. Diagnostics never echo
// source values, including errors returned by the TOML decoder.
func Parse(r io.Reader) (Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConfigBytes+1))
	if err != nil {
		return Config{}, errors.New("cannot read configuration file")
	}
	if len(data) > maxConfigBytes {
		return Config{}, errors.New("configuration exceeds 1 MiB limit")
	}
	c := defaults()
	metadata, err := toml.Decode(string(data), &c)
	if err != nil {
		return Config{}, errors.New("invalid TOML configuration or field type")
	}
	if len(metadata.Undecoded()) != 0 {
		return Config{}, errors.New("unknown configuration key or section")
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return c, nil
}
