package statefile

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestStrictJSON(t *testing.T) {
	for _, input := range []string{`{"n":1,"n":2}`, `{"n":1} {"n":2}`, `{"unknown":1}`, `{"n":"SECRET_PLACEHOLDER"}`, `null null`, `{`, strings.Repeat("[", 40) + strings.Repeat("]", 40)} {
		var dst struct {
			N int `json:"n"`
		}
		err := Decode([]byte(input), &dst)
		if err == nil || strings.Contains(err.Error(), "SECRET_PLACEHOLDER") {
			t.Fatalf("unsafe parse error: %v", err)
		}
	}
	var dst struct {
		N int `json:"n"`
	}
	if err := Decode([]byte(`{"n":2}`), &dst); err != nil || dst.N != 2 {
		t.Fatal(err)
	}
	path := t.TempDir() + "/state.json"
	if err := ReadJSON(path, 10, &dst); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"n":12345}`), 0600); err != nil {
		t.Fatal(err)
	}
	if ReadJSON(path, 4, &dst) == nil {
		t.Fatal("oversized input accepted")
	}
}
