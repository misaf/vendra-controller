package compose

import (
	"context"
	"io"

	"github.com/misaf/vendra-controller/internal/docker"
)

type Project struct {
	Docker             docker.Service
	Dir, Name, EnvFile string
	Files              []string
}

func (p Project) run(ctx context.Context, stdin io.Reader, args ...string) error {
	return p.Docker.Compose(ctx, p.Dir, p.Name, p.EnvFile, p.Files, args, stdin)
}
func (p Project) Pull(ctx context.Context, services ...string) error {
	return p.run(ctx, nil, append([]string{"pull"}, services...)...)
}
func (p Project) Up(ctx context.Context, wait bool) error {
	args := []string{"up", "-d", "--remove-orphans"}
	if wait {
		args = append(args, "--wait")
	}
	return p.run(ctx, nil, args...)
}
func (p Project) Down(ctx context.Context) error    { return p.run(ctx, nil, "down") }
func (p Project) Restart(ctx context.Context) error { return p.run(ctx, nil, "restart") }
func (p Project) PS(ctx context.Context) error      { return p.run(ctx, nil, "ps") }
func (p Project) Logs(ctx context.Context, stdin io.Reader, target string) error {
	args := []string{"logs", "-f"}
	if target != "" {
		args = append(args, target)
	}
	return p.run(ctx, stdin, args...)
}
func (p Project) Validate(ctx context.Context) error { return p.run(ctx, nil, "config", "--quiet") }
