package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/controller"
	"github.com/misaf/vendra-controller/internal/docker"
	"github.com/misaf/vendra-controller/internal/hosts"
	"github.com/misaf/vendra-controller/internal/process"
	"github.com/misaf/vendra-controller/internal/property"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type options struct {
	configPath, stateDir, logFormat string
	verbose, noColor, noPull        bool
}
type application struct {
	options        *options
	version        string
	runner         process.Runner
	stdin          io.Reader
	stdout, stderr io.Writer
}

func New(version string) *cobra.Command {
	return NewWith(version, process.ExecRunner{}, os.Stdin, os.Stdout, os.Stderr)
}
func NewWith(version string, runner process.Runner, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	o := &options{}
	app := &application{options: o, version: version, runner: runner, stdin: stdin, stdout: stdout, stderr: stderr}
	root := &cobra.Command{Use: "vendra", Short: "Vendra infrastructure controller", SilenceUsage: true, SilenceErrors: true}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&o.configPath, "config", "", "configuration file")
	root.PersistentFlags().StringVar(&o.stateDir, "state-dir", "", "runtime state directory")
	root.PersistentFlags().StringVar(&o.logFormat, "log-format", "text", "text or json logs")
	root.PersistentFlags().BoolVarP(&o.verbose, "verbose", "v", false, "enable verbose logs")
	root.PersistentFlags().BoolVar(&o.noColor, "no-color", false, "disable color output")
	root.PersistentFlags().BoolVar(&o.noPull, "no-pull", false, "use images already in the local Docker daemon instead of pulling")
	root.AddCommand(app.initCommand(), app.stackCommand(), app.propertyCommand(), &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, _ []string) error { fmt.Fprintln(cmd.OutOrStdout(), version); return nil }})
	root.AddCommand(completionCommand(root))
	return root
}
func (a *application) load() (*controller.Controller, error) {
	cfg, err := config.Load(a.options.configPath)
	if err != nil {
		return nil, err
	}
	if a.options.stateDir != "" {
		cfg.StateDir = a.options.stateDir
	}
	service := docker.Service{Runner: a.runner}
	c := controller.New(cfg, service)
	c.NoPull = a.options.noPull
	c.Properties.NoPull = a.options.noPull
	return c, nil
}
func (a *application) initCommand() *cobra.Command {
	return &cobra.Command{Use: "init", Short: "Initialize controller state", RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		if err = c.Init(); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "initialized %s\n", c.Config.StateDir)
		return nil
	}}
}
func (a *application) stackCommand() *cobra.Command {
	stack := &cobra.Command{Use: "stack", Short: "Manage the Vendra stack"}
	stack.AddCommand(simple("up", "Start the stack", func(ctx context.Context, c *controller.Controller) error { return c.Up(ctx) }, a), simple("down", "Stop the stack", func(ctx context.Context, c *controller.Controller) error { return c.Down(ctx) }, a), simple("restart", "Restart the stack", func(ctx context.Context, c *controller.Controller) error { return c.Restart(ctx) }, a), simple("ps", "Show Compose state", func(ctx context.Context, c *controller.Controller) error { return c.PS(ctx) }, a))
	stack.AddCommand(&cobra.Command{Use: "logs [target]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		target := "php"
		if len(args) > 0 {
			target = args[0]
		}
		return c.Logs(cmd.Context(), target, a.stdin)
	}})
	stack.AddCommand(&cobra.Command{Use: "urls", RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		values := c.URLs()
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s\n", key, values[key])
		}
		return nil
	}})
	var write bool
	hostsCommand := &cobra.Command{Use: "hosts", RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		names := c.HostNames()
		if !write {
			fmt.Fprintf(cmd.OutOrStdout(), "127.0.0.1 %s\n", strings.Join(names, " "))
			return nil
		}
		return hosts.Write("/etc/hosts", names)
	}}
	hostsCommand.Flags().BoolVar(&write, "write", false, "update /etc/hosts")
	stack.AddCommand(hostsCommand)
	var asJSON bool
	status := &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		err = c.Docker.Available(cmd.Context())
		if asJSON {
			value := map[string]any{"docker": err == nil, "state_dir": c.Config.StateDir}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(value)
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "docker: running")
		return c.PS(cmd.Context())
	}}
	status.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	stack.AddCommand(status)
	return stack
}
func (a *application) propertyCommand() *cobra.Command {
	root := &cobra.Command{Use: "property", Short: "Manage storefront properties"}
	root.AddCommand(a.propertyWrite("add", false), a.propertyWrite("render", true))
	for _, item := range []struct {
		name string
		run  func(context.Context, *controller.Controller, string) error
	}{
		{"up", func(ctx context.Context, c *controller.Controller, slug string) error {
			return c.Properties.Up(ctx, slug)
		}},
		{"down", func(ctx context.Context, c *controller.Controller, slug string) error {
			return c.Properties.Down(ctx, slug)
		}},
		{"restart", func(ctx context.Context, c *controller.Controller, slug string) error {
			return c.Properties.Restart(ctx, slug)
		}},
	} {
		item := item
		root.AddCommand(&cobra.Command{Use: item.name + " <slug>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			c, err := a.load()
			if err != nil {
				return err
			}
			return item.run(cmd.Context(), c, args[0])
		}})
	}
	var yes bool
	remove := &cobra.Command{Use: "remove <slug>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			fmt.Fprintf(cmd.OutOrStdout(), "Remove property %s? [y/N] ", args[0])
			var answer string
			fmt.Fscan(a.stdin, &answer)
			if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
				fmt.Fprintln(cmd.OutOrStdout(), "aborted")
				return nil
			}
		}
		c, err := a.load()
		if err != nil {
			return err
		}
		if err = c.Properties.Remove(cmd.Context(), args[0]); err != nil {
			return err
		}
		if c.Config.CertificateMode == "self-signed" {
			return c.RegenerateCertificate()
		}
		return nil
	}}
	remove.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	root.AddCommand(remove)
	root.AddCommand(a.propertyList(), a.propertySync())
	return root
}

// propertyList reports drift in both directions: registered but not rendered on
// this host, and rendered but absent from the registry — the second matters
// because a restore driven by the registry would silently skip it.
func (a *application) propertyList() *cobra.Command {
	return &cobra.Command{Use: "ls", Short: "List the fleet and any registry drift", RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		fleet, err := c.Properties.Fleet()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if len(fleet) == 0 {
			fmt.Fprintln(out, "no properties")
			return nil
		}
		writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(writer, "SLUG\tDOMAIN\tSTATE\tIMAGE")
		for _, entry := range fleet {
			state := "ok"
			switch {
			case entry.Registry && !entry.Rendered:
				state = "not rendered"
			case !entry.Registry && entry.Rendered:
				state = "unregistered"
			}
			domain, image := entry.Domain, entry.Image
			if domain == "" {
				domain = "-"
			}
			if image == "" {
				image = "-"
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", entry.Slug, domain, state, image)
		}
		return writer.Flush()
	}}
}

func (a *application) propertySync() *cobra.Command {
	return &cobra.Command{Use: "sync", Short: "Re-render every registered property", RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		unresolved, err := c.Properties.Sync(cmd.Context())
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if len(unresolved) == 0 {
			fmt.Fprintln(out, "synced")
			return nil
		}
		// Not an error: the registry holds no configuration by design, so these
		// have to come back through Vendra rather than from this host.
		fmt.Fprintf(out, "synced, except %s — no stored configuration.\n", strings.Join(unresolved, ", "))
		fmt.Fprintln(out, "Re-provision them from Vendra, or re-run `property render` with --configuration.")
		return nil
	}}
}
func (a *application) propertyWrite(name string, idempotent bool) *cobra.Command {
	var image, theme, configuration, encoded string
	cmd := &cobra.Command{Use: name + " <slug> <domain>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		if image == "" {
			image = c.Config.Images.Storefront
		}
		if configuration != "" && encoded != "" {
			return fmt.Errorf("--configuration and --configuration-base64 are mutually exclusive")
		}
		if configuration != "" {
			data, err := os.ReadFile(configuration)
			if err != nil {
				return err
			}
			encoded = base64.StdEncoding.EncodeToString(data)
		}
		spec := property.Spec{Slug: args[0], Domain: args[1], Image: image, Theme: theme, ConfigurationBase64: encoded}
		if idempotent {
			err = c.Properties.Render(cmd.Context(), spec)
		} else {
			err = c.Properties.Add(cmd.Context(), spec)
		}
		if err != nil {
			return err
		}
		if c.Config.CertificateMode == "self-signed" {
			return c.RegenerateCertificate()
		}
		return nil
	}}
	cmd.Flags().StringVar(&image, "image", "", "storefront image")
	cmd.Flags().StringVar(&theme, "theme", "default", "storefront theme")
	cmd.Flags().StringVar(&configuration, "configuration", "", "JSON configuration file")
	cmd.Flags().StringVar(&encoded, "configuration-base64", "", "base64 JSON configuration")
	return cmd
}
func simple(name, description string, run func(context.Context, *controller.Controller) error, a *application) *cobra.Command {
	return &cobra.Command{Use: name, Short: description, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := a.load()
		if err != nil {
			return err
		}
		return run(cmd.Context(), c)
	}}
}
func completionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return root.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return root.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return root.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return root.GenPowerShellCompletion(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unsupported shell %s", args[0])
		}
	}}
}

func WriteExampleConfig(path string) error {
	cfg := config.Defaults()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(path), data, 0o600)
}
func Logger(format string, verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	options := &slog.HandlerOptions{Level: level}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, options))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, options))
}
