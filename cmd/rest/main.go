package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/repomz/rest_generator/internal/appgen"
	"github.com/repomz/rest_generator/internal/config"
	"github.com/repomz/rest_generator/internal/sqlcconfig"
)

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
	case "config":
		return runConfig(args[1:])
	case "app":
		return runApp(args[1:])
	case "sqlc":
		return runSQLC(args[1:])
	default:
		return usageError()
	}
}

func runConfig(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	if args[0] == "sqlc" {
		if len(args) < 2 || args[1] != "generate" {
			return usageError()
		}
		out, err := parseOutputDir(args[2:])
		if err != nil {
			return err
		}
		if err := sqlcconfig.ValidateProject(out); err != nil {
			return err
		}
		if err := config.GenerateForSQLC(filepath.Join(out, "rest_config")); err != nil {
			return err
		}
		return sqlcconfig.GenerateProject(out)
	}
	if args[0] != "generate" {
		return usageError()
	}
	out := "rest_config"
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-out":
			i++
			if i >= len(args) {
				return fmt.Errorf("-out requires a path")
			}
			out = args[i]
		default:
			return fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return config.Generate(out)
}

func runSQLC(args []string) error {
	if len(args) < 2 || args[0] != "example" || args[1] != "generate" {
		return usageError()
	}
	configDir := "rest_config"
	out := "."
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "-config":
			i++
			if i >= len(args) {
				return fmt.Errorf("-config requires a path")
			}
			configDir = args[i]
		case "-out":
			i++
			if i >= len(args) {
				return fmt.Errorf("-out requires a path")
			}
			out = args[i]
		default:
			return fmt.Errorf("unknown argument %q", args[i])
		}
	}
	bundle, err := config.Load(configDir)
	if err != nil {
		return err
	}
	if bundle.SQL == nil || !bundle.SQL.SQLC.Example.Bool() {
		return fmt.Errorf("sqlc example generation is disabled in %s/sqlc_rest.yaml", configDir)
	}
	return sqlcconfig.GenerateExample(out)
}

func parseOutputDir(args []string) (string, error) {
	out := "."
	for i := 0; i < len(args); i++ {
		if args[i] != "-out" {
			return "", fmt.Errorf("unknown argument %q", args[i])
		}
		i++
		if i >= len(args) {
			return "", fmt.Errorf("-out requires a path")
		}
		out = args[i]
	}
	return out, nil
}

func runApp(args []string) error {
	if len(args) == 0 || args[0] != "generate" {
		return usageError()
	}
	return runGenerate(args[1:])
}

func runGenerate(args []string) error {
	configDir, err := parseConfigDir(args)
	if err != nil {
		return err
	}
	appGenerator := appgen.New(appgen.DefaultRegistry()...)
	return appGenerator.Generate(configDir)
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

func usageError() error {
	return fmt.Errorf("usage: rest config generate [-out rest_config] | rest config sqlc generate [-out .] | rest sqlc example generate [-config rest_config] [-out .] | rest app generate [-config rest_config]")
}
