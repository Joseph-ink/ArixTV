package config

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDefaultsAndExample(t *testing.T) {
	c, err := Parse(strings.NewReader("# Defaults are valid, with all external modes disabled.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 8787 || c.Server.LoopbackAddress != "127.0.0.1" ||
		c.Refresh.Interval != "6h" || c.Refresh.GlobalConcurrency != 8 ||
		c.Refresh.ProviderConcurrency != 3 || c.LAN.Enabled || c.Remote.Enabled || c.Relay.Enabled {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if _, err := Load("../../config.example.toml"); err != nil {
		t.Fatalf("checked-in example: %v", err)
	}
}

func TestParseNetworkConfiguration(t *testing.T) {
	c, err := Parse(strings.NewReader(`
[server]
loopback_address = "::1"
port = 9876
[lan]
enabled = true
listen_addresses = ["192.168.50.10", "fd00::10"]
allow_networks = ["192.168.50.0/24", "fd00::/64"]
[remote]
enabled = true
public_origin = 'https://tv.example.com:8443'
trusted_proxies = ["::1/128"]
[providers]
enabled = [
  "lg-channels", # TOML multiline lists and comments
  "xumo",
]
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 9876 || !c.LAN.Enabled || !c.Remote.Enabled || len(c.Providers.Enabled) != 2 {
		t.Fatalf("configuration did not decode: %+v", c)
	}
}

func TestRejectInvalidConfiguration(t *testing.T) {
	tests := map[string]string{
		"syntax":        `[server`,
		"duplicate key": "[server]\nport=8787\nport=8788",
		"wrong type": `[server]
port="8787"`,
		"unknown key":        "[server]\nprot=8787",
		"unknown section":    "[secrets]\nvalue='PLACEHOLDER'",
		"public listener":    "[server]\nloopback_address='0.0.0.0'",
		"nonloopback":        "[server]\nloopback_address='192.168.50.10'",
		"zero port":          "[server]\nport=0",
		"large port":         "[server]\nport=65536",
		"empty LAN":          "[lan]\nenabled=true",
		"wildcard LAN":       "[lan]\nlisten_addresses=['0.0.0.0']",
		"IPv6 wildcard LAN":  "[lan]\nlisten_addresses=['::']",
		"loopback LAN":       "[lan]\nlisten_addresses=['127.0.0.1']",
		"multicast LAN":      "[lan]\nlisten_addresses=['224.0.0.1']",
		"invalid LAN IP":     "[lan]\nlisten_addresses=['tv.example.com']",
		"invalid LAN CIDR":   "[lan]\nallow_networks=['192.168.50.0/33']",
		"invalid proxy CIDR": "[remote]\ntrusted_proxies=['localhost']",
		"empty remote":       "[remote]\nenabled=true",
		"empty proxy list":   "[remote]\nenabled=true\npublic_origin='https://tv.example.com'\ntrusted_proxies=[]",
		"HTTP remote":        "[remote]\npublic_origin='http://tv.example.com'",
		"origin userinfo":    "[remote]\npublic_origin='https://user:PLACEHOLDER@tv.example.com'",
		"origin path":        "[remote]\npublic_origin='https://tv.example.com/direct'",
		"origin query":       "[remote]\npublic_origin='https://tv.example.com?token=PLACEHOLDER'",
		"origin fragment":    "[remote]\npublic_origin='https://tv.example.com#fragment'",
		"origin bad port":    "[remote]\npublic_origin='https://tv.example.com:65536'",
		"relative path":      "[remote]\ndirect_path='direct'",
		"root path":          "[remote]\ndirect_path='/'",
		"unclean path":       "[remote]\ndirect_path='/a/../direct'",
		"trailing slash":     "[remote]\ndirect_path='/direct/'",
		"escaped path":       "[remote]\ndirect_path='/%2e%2e/direct'",
		"path query":         "[remote]\ndirect_path='/direct?x=1'",
		"negative interval":  "[refresh]\ninterval='-1h'",
		"invalid interval":   "[refresh]\ninterval='six hours'",
		"zero concurrency":   "[refresh]\nglobal_concurrency=0",
		"negative workers":   "[refresh]\nprovider_concurrency=-1",
		"duplicate provider": "[providers]\nenabled=['xumo','xumo']",
		"invalid provider":   "[providers]\nenabled=['../xumo']",
		"empty provider":     "[providers]\nenabled=['']",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(input)); err == nil {
				t.Fatal("accepted invalid configuration")
			}
		})
	}
}

func TestErrorsDoNotEchoInput(t *testing.T) {
	const placeholder = "SECRET_PLACEHOLDER_DO_NOT_ECHO"
	for _, input := range []string{
		"[server]\nport='" + placeholder + "'",
		"[remote]\npublic_origin='https://user:" + placeholder + "@tv.example.com'",
		placeholder + " = true",
		"[" + placeholder,
	} {
		_, err := Parse(strings.NewReader(input))
		if err == nil || strings.Contains(err.Error(), placeholder) {
			t.Fatalf("unsafe or missing error: %v", err)
		}
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("PRIVATE_PLACEHOLDER") }

func TestInputFailures(t *testing.T) {
	if _, err := Parse(strings.NewReader(strings.Repeat(" ", maxConfigBytes+1))); err == nil {
		t.Fatal("accepted oversized configuration")
	}
	if _, err := Parse(brokenReader{}); err == nil || strings.Contains(err.Error(), "PRIVATE_PLACEHOLDER") {
		t.Fatalf("unsafe read error: %v", err)
	}
	if _, err := Load(t.TempDir() + "/missing.toml"); err == nil {
		t.Fatal("accepted missing file")
	}
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("accepted directory")
	}
}

func FuzzParse(f *testing.F) {
	f.Add("[server]\nport=8787")
	f.Add("[remote]\npublic_origin='https://tv.example.com'")
	f.Add("\xff")
	f.Fuzz(func(t *testing.T, input string) {
		c, err := Parse(strings.NewReader(input))
		if err == nil {
			if err := c.Validate(); err != nil {
				t.Fatalf("Parse returned an invalid configuration: %v", err)
			}
		}
	})
}

func TestLoadFile(t *testing.T) {
	file := t.TempDir() + "/config.toml"
	if err := os.WriteFile(file, []byte("[server]\nport=9999"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(file)
	if err != nil || c.Server.Port != 9999 {
		t.Fatalf("Load: port=%d, err=%v", c.Server.Port, err)
	}
}
