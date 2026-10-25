package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	valid, invalid := dir+"/valid.toml", dir+"/invalid.toml"
	for name, contents := range map[string]string{
		valid:   "[server]\nport=8787",
		invalid: "[server]\nport='SECRET_PLACEHOLDER_DO_NOT_ECHO'",
	} {
		if err := os.WriteFile(name, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	build := BuildInfo{Version: "v0.0.0-test", Commit: "test-commit", BuildDate: "test-date"}
	tests := []struct {
		name string
		args []string
		code int
		out  string
	}{
		{"version", []string{"version"}, 0, "arixtv v0.0.0-test\ncommit: test-commit\nbuild date: test-date\n"},
		{"valid", []string{"config", "validate", "--config", valid}, 0, "configuration valid\n"},
		{"invalid", []string{"config", "validate", "--config", invalid}, 1, ""},
		{"missing file", []string{"config", "validate", "--config", dir + "/missing"}, 1, ""},
		{"help", []string{"--help"}, 0, usage},
		{"config help", []string{"config", "validate", "--help"}, 0, usage},
		{"no command", nil, 2, ""},
		{"future command", []string{"serve"}, 2, ""},
		{"version extra", []string{"version", "extra"}, 2, ""},
		{"missing subcommand", []string{"config"}, 2, ""},
		{"unknown subcommand", []string{"config", "show"}, 2, ""},
		{"missing flag value", []string{"config", "validate", "--config"}, 2, ""},
		{"unknown flag", []string{"config", "validate", "--token=SECRET_PLACEHOLDER_DO_NOT_ECHO"}, 2, ""},
		{"empty path", []string{"config", "validate", "--config="}, 2, ""},
		{"positional", []string{"config", "validate", valid}, 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr, build)
			if code != tt.code || stdout.String() != tt.out {
				t.Fatalf("code=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
			if (code != 0) != (stderr.Len() != 0) {
				t.Fatalf("unexpected stderr: %q", stderr.String())
			}
			if strings.Contains(stdout.String()+stderr.String(), "SECRET_PLACEHOLDER_DO_NOT_ECHO") {
				t.Fatal("input appeared in diagnostics")
			}
		})
	}
}

func TestDefaultConfigPath(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"config", "validate"}, &stdout, &stderr, BuildInfo{}); code != 1 {
		t.Fatalf("missing default file returned %d", code)
	}
	if err := os.WriteFile("config.toml", []byte("[server]\nport=8787"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"config", "validate"}, &stdout, &stderr, BuildInfo{}); code != 0 {
		t.Fatalf("local default returned %d: %s", code, stderr.String())
	}
	if stdout.String() != "configuration valid\n" || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
