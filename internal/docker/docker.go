package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/misaf/vendra-controller/internal/process"
)

type Service struct{ Runner process.Runner }

func (s Service) run(ctx context.Context, args ...string) error {
	return s.Runner.Run(ctx, process.Request{Name: "docker", Args: args})
}
func (s Service) Available(ctx context.Context) error {
	if err := s.run(ctx, "info"); err != nil {
		return fmt.Errorf("docker unavailable: %w", err)
	}
	return nil
}
func (s Service) EnsureNetwork(ctx context.Context, name string) error {
	// The probe is expected to fail the first time, so its output is discarded:
	// otherwise every fresh start prints "network traefik-public not found",
	// which reads as a failure immediately before the network is created.
	probe := process.Request{Name: "docker", Args: []string{"network", "inspect", name}, Stdout: io.Discard, Stderr: io.Discard}
	if err := s.Runner.Run(ctx, probe); err == nil {
		return nil
	}
	return s.run(ctx, "network", "create", name)
}
func (s Service) Inspect(ctx context.Context, name string) error { return s.run(ctx, "inspect", name) }
func (s Service) Compose(ctx context.Context, dir, project, envFile string, files, args []string, stdin io.Reader) error {
	command := []string{"compose", "-p", project}
	if envFile != "" {
		command = append(command, "--env-file", envFile)
	}
	for _, file := range files {
		command = append(command, "-f", file)
	}
	command = append(command, args...)
	return s.Runner.Run(ctx, process.Request{Dir: dir, Name: "docker", Args: command, Stdin: stdin})
}
