package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Request struct {
	Dir, Name      string
	Args           []string
	Env            []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}
type Runner interface {
	Run(context.Context, Request) error
}
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, request Request) error {
	cmd := exec.CommandContext(ctx, request.Name, request.Args...)
	cmd.Dir, cmd.Stdin = request.Dir, request.Stdin
	cmd.Stdout, cmd.Stderr = request.Stdout, request.Stderr
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	cmd.Env = append(os.Environ(), request.Env...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s %v: %w", request.Name, request.Args, err)
	}
	return nil
}
