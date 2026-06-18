package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/repomz/rest/internal/appgen"
	"github.com/repomz/rest/internal/config"
	"github.com/repomz/rest/internal/selfupdate"
	"github.com/repomz/rest/internal/sqlcconfig"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "generate":
		return runGenerate(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "version":
		fmt.Println(currentVersion())
		return nil
	default:
		return usageError()
	}
}

func runInit(args []string) error {
	options, err := parseInitOptions(args)
	if err != nil {
		return err
	}
	if options.withSQLC && options.withExample {
		return fmt.Errorf("use either --sqlc or --example, not both")
	}
	if options.withSQLC {
		if err := sqlcconfig.ValidateProject(options.out); err != nil {
			return err
		}
	}
	if options.withExample {
		if err := sqlcconfig.ValidateExample(options.out); err != nil {
			return err
		}
	}
	configDir := filepath.Join(options.out, "rest_config")
	if options.withExample {
		if err := config.GenerateForExample(configDir); err != nil {
			return err
		}
	} else if options.withSQLC {
		if err := config.GenerateForSQLC(configDir); err != nil {
			return err
		}
	} else {
		if err := config.Generate(configDir); err != nil {
			return err
		}
	}
	if options.withSQLC {
		if err := sqlcconfig.RemoveExample(options.out); err != nil {
			return err
		}
		if err := sqlcconfig.GenerateProject(options.out); err != nil {
			return err
		}
	}
	if options.withExample {
		if err := sqlcconfig.GenerateExample(options.out); err != nil {
			return err
		}
	}
	return nil
}

type initOptions struct {
	out         string
	withSQLC    bool
	withExample bool
}

func parseInitOptions(args []string) (initOptions, error) {
	options := initOptions{out: "."}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--sqlc":
			options.withSQLC = true
		case "--example":
			options.withExample = true
		case "-out", "--out":
			i++
			if i >= len(args) {
				return initOptions{}, fmt.Errorf("%s requires a path", args[i-1])
			}
			options.out = args[i]
		default:
			return initOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return options, nil
}

func runGenerate(args []string) error {
	configDir, err := parseConfigDir(args)
	if err != nil {
		return err
	}
	appGenerator := appgen.New(appgen.DefaultRegistry()...)
	return appGenerator.Generate(configDir)
}

func runUpdate(args []string) error {
	options, err := parseUpdateOptions(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := selfupdate.Update(ctx, selfupdate.Options{
		CurrentVersion: currentVersion(),
		TargetVersion:  options.version,
		Force:          options.force,
		Stdout:         os.Stdout,
	})
	if err != nil {
		return err
	}
	if !result.Updated {
		fmt.Fprintf(os.Stdout, "rest is already up to date (%s)\n", result.Version)
		return nil
	}
	fmt.Fprintf(os.Stdout, "updated rest from %s to %s\n", result.PreviousVersion, result.Version)
	return nil
}

func parseConfigDir(args []string) (string, error) {
	configDir := "rest_config"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-config":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("-config requires a path")
			}
			configDir = args[i]
		default:
			return "", fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return configDir, nil
}

type updateOptions struct {
	version string
	force   bool
}

func parseUpdateOptions(args []string) (updateOptions, error) {
	var options updateOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version":
			i++
			if i >= len(args) {
				return updateOptions{}, fmt.Errorf("--version requires a release tag")
			}
			options.version = args[i]
		case "--force":
			options.force = true
		default:
			return updateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return options, nil
}

func usageError() error {
	return fmt.Errorf("usage: rest init [--sqlc|--example] [--out .] | rest generate [-config rest_config] | rest update [--version vX.Y.Z] [--force] | rest version")
}

func currentVersion() string {
	buildVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		buildVersion = info.Main.Version
	}
	return resolveVersion(version, buildVersion)
}

func resolveVersion(linkerVersion, buildVersion string) string {
	if linkerVersion != "" && linkerVersion != "dev" {
		return linkerVersion
	}
	if buildVersion != "" && buildVersion != "(devel)" {
		return buildVersion
	}
	return "dev"
}
