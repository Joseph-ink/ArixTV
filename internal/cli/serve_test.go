package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeCLI(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	contents := "[storage]\ndata_dir='" + filepath.Join(root, "data") + "'\nstate_dir='" + filepath.Join(root, "state") + "'\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		args []string
		code int
	}{{[]string{"serve", "--config", path}, 0}, {[]string{"serve", "--help"}, 0}, {[]string{"serve", "--config", filepath.Join(root, "missing")}, 1}, {[]string{"serve", "--unknown=PRIVATE_PLACEHOLDER"}, 2}, {[]string{"serve", "extra"}, 2}} {
		var out, errout bytes.Buffer
		if code := RunContext(ctx, tc.args, &out, &errout, BuildInfo{Version: "test"}); code != tc.code {
			t.Fatalf("serve exit %d want %d: %s", code, tc.code, errout.String())
		}
		if strings.Contains(out.String()+errout.String(), "PRIVATE_PLACEHOLDER") {
			t.Fatal("secret argument echoed")
		}
	}
	for _, name := range []string{"data", "state"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatal("cancelled serve created state")
		}
	}
}
