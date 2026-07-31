package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertPreservesUnknownValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# user comment\nCUSTOM=value\nDOMAIN=old.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Upsert(path, map[string]string{"DOMAIN": "new.test", "ROUTER_NAME": "new"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, expected := range []string{"# user comment", "CUSTOM=value", "DOMAIN=new.test", "ROUTER_NAME=new"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
}
