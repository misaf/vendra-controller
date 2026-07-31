package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/misaf/vendra-controller/internal/process"
)

type noProcess struct{}

func (noProcess) Run(_ context.Context, _ process.Request) error { return nil }

func TestURLsCommandDoesNotRequireDocker(t *testing.T) {
	var output bytes.Buffer
	root := NewWith("test", noProcess{}, strings.NewReader(""), &output, &output)
	root.SetArgs([]string{"--state-dir", t.TempDir(), "stack", "urls"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "api") || !strings.Contains(output.String(), "vendra.test") {
		t.Fatalf("unexpected output: %s", output.String())
	}
}
