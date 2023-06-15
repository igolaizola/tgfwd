package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"

	"github.com/igolaizola/tgfwd"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

// Build flags
var version = ""
var commit = ""
var date = ""

func main() {
	// Create signal based context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Launch command
	cmd := newCommand()
	if err := cmd.ParseAndRun(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func newCommand() *ffcli.Command {
	fs := flag.NewFlagSet("tgfwd", flag.ExitOnError)

	return &ffcli.Command{
		ShortUsage: "tgfwd [flags] <subcommand>",
		FlagSet:    fs,
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
		Subcommands: []*ffcli.Command{
			newVersionCommand(),
			newLoginCommand(),
			newListCommand(),
			newRunCommand(),
		},
	}
}

func newVersionCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "tgfwd version",
		ShortHelp:  "print version",
		Exec: func(ctx context.Context, args []string) error {
			v := version
			if v == "" {
				if buildInfo, ok := debug.ReadBuildInfo(); ok {
					v = buildInfo.Main.Version
				}
			}
			if v == "" {
				v = "dev"
			}
			versionFields := []string{v}
			if commit != "" {
				versionFields = append(versionFields, commit)
			}
			if date != "" {
				versionFields = append(versionFields, date)
			}
			fmt.Println(strings.Join(versionFields, " "))
			return nil
		},
	}
}

func newLoginCommand() *ffcli.Command {
	cmd := "login"
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	_ = fs.String("config", "", "config file (optional)")

	var cfg tgfwd.Config
	fs.StringVar(&cfg.Phone, "phone", "", "phone")
	fs.IntVar(&cfg.ID, "id", 0, "app id")
	fs.StringVar(&cfg.Hash, "hash", "", "app hash")
	fs.StringVar(&cfg.SessionPath, "session", "", "session file")
	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("tgfwd %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ff.PlainParser),
			ff.WithEnvVarPrefix("TGFWD"),
		},
		ShortHelp: fmt.Sprintf("tgfwd %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			return tgfwd.Login(ctx, &cfg)
		},
	}
}

func newListCommand() *ffcli.Command {
	cmd := "list"
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	_ = fs.String("config", "", "config file (optional)")

	var cfg tgfwd.Config
	fs.IntVar(&cfg.ID, "id", 0, "app id")
	fs.StringVar(&cfg.Hash, "hash", "", "app hash")
	fs.StringVar(&cfg.SessionPath, "session", "", "session file")
	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("tgfwd %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ff.PlainParser),
			ff.WithEnvVarPrefix("TGFWD"),
		},
		ShortHelp: fmt.Sprintf("tgfwd %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			return tgfwd.List(ctx, &cfg)
		},
	}
}

func newRunCommand() *ffcli.Command {
	cmd := "run"
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	_ = fs.String("config", "", "config file (optional)")

	var cfg tgfwd.Config
	fs.IntVar(&cfg.ID, "id", 0, "app id")
	fs.StringVar(&cfg.Hash, "hash", "", "app hash")
	fs.StringVar(&cfg.SessionPath, "session", "", "session file")
	forwardsVar(fs, &cfg.Forwards, "fwd", "fwd from-id:to-id (one or more)")
	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("tgfwd %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ff.PlainParser),
			ff.WithEnvVarPrefix("TGFWD"),
		},
		ShortHelp: fmt.Sprintf("tgfwd %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			return tgfwd.Run(ctx, &cfg)
		},
	}
}

func forwardsVar(fs *flag.FlagSet, p *[][2]int64, name, usage string) {
	fs.Var((*forwardArray)(p), name, usage)
}

type forwardArray [][2]int64

func (s *forwardArray) String() string {
	var ss []string
	for _, v := range *s {
		ss = append(ss, fmt.Sprintf("%d:%d", v[0], v[1]))
	}
	return strings.Join(ss, ",")
}

func (s *forwardArray) Set(v string) error {
	var i1, i2 int64
	if _, err := fmt.Sscanf(v, "%d:%d", &i1, &i2); err != nil {
		return err
	}
	if i1 == 0 || i2 == 0 {
		return fmt.Errorf("invalid %d:%d", i1, i2)
	}
	*s = append(*s, [2]int64{i1, i2})
	return nil
}
