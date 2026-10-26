package config

import (
	"strings"
	"testing"
)

func TestHTTPConfiguration(t *testing.T) {
	c, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTP.MaxConnections != 128 || c.HTTP.ResolverConcurrency != 8 || c.HTTP.ReloadInterval != "5s" {
		t.Fatal("HTTP defaults")
	}
	for _, bad := range []string{
		"[http]\nread_header_timeout='0s'", "[http]\nresolve_timeout='30s'", "[http]\nresolver_concurrency=0", "[http]\nmax_connections=0", "[http]\nrate_max_clients=0", "[http]\nrate_burst=0", "[http]\nplaylist_per_minute=0", "[http]\nmax_header_bytes=0", "[http]\nunknown=1", "[http]\nreload_interval='-1s'",
		"[lan]\nenabled=true\nlisten_addresses=['192.168.50.10']\nallow_networks=['192.168.50.0/24']",
		"[lan]\npublic_origin='https://tv.example.com'", "[lan]\npublic_origin='http://tv.example.com:65536'", "[lan]\npublic_origin='http://user:PLACEHOLDER@tv.example.com'", "[lan]\npublic_origin='http://tv.example.com/path'",
		"[remote]\ndirect_path='/internal/direct'", "[remote]\ndirect_path='/healthz'", "[remote]\ndirect_path='/direçt'",
	} {
		if _, err := Parse(strings.NewReader(bad)); err == nil {
			t.Fatalf("invalid HTTP configuration accepted: %s", bad)
		}
	}
	if _, err := Parse(strings.NewReader("[http]\nresolver_concurrency=2\nread_header_timeout='2s'\n[lan]\npublic_origin='http://[fd00::1]:8787'")); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPPolicyInputBounds(t *testing.T) {
	for _, input := range []string{"[http]\nrate_client_ttl='1ms'", "[remote]\npublic_origin='https://" + strings.Repeat("x", 2048) + "'"} {
		if _, err := Parse(strings.NewReader(input)); err == nil {
			t.Fatal("unbounded policy input accepted")
		}
	}
}
