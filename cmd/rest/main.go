package main

import (
	"context"
	"fmt"
	"io"
	"os"
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
	case "gen":
		return runGen(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "changelog":
		return runChangelog(args[1:])
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
	switch options.example {
	case "sql":
		if err := sqlcconfig.ValidateExample("."); err != nil {
			return err
		}
	case "mongo":
	default:
		if err := sqlcconfig.ValidateProject("."); err != nil {
			return err
		}
	}
	configDir := "rest_config"
	switch options.example {
	case "sql":
		if err := config.GenerateForExample(configDir); err != nil {
			return err
		}
	case "mongo":
		if err := config.GenerateForMongoExample(configDir); err != nil {
			return err
		}
	default:
		if err := config.GenerateForSQLC(configDir); err != nil {
			return err
		}
	}
	switch options.example {
	case "sql":
		if err := sqlcconfig.GenerateExample("."); err != nil {
			return err
		}
	case "mongo":
		return nil
	default:
		if err := sqlcconfig.RemoveExample("."); err != nil {
			return err
		}
		if err := sqlcconfig.GenerateProject("."); err != nil {
			return err
		}
	}
	return nil
}

type initOptions struct {
	example string
}

func parseInitOptions(args []string) (initOptions, error) {
	var options initOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--example":
			i++
			if i >= len(args) {
				return initOptions{}, fmt.Errorf("--example requires sql or mongo")
			}
			switch args[i] {
			case "sql", "mongo":
				options.example = args[i]
			default:
				return initOptions{}, fmt.Errorf("--example supports only sql or mongo")
			}
		default:
			return initOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return options, nil
}

func runGen(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown argument %q", args[0])
	}
	configDir := "rest_config"
	if err := config.ValidateYAMLTree(configDir); err != nil {
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
	printUpdateResult(os.Stdout, result)
	return nil
}

func printUpdateResult(w io.Writer, result selfupdate.Result) {
	fmt.Fprintln(w, "Updating rest")
	fmt.Fprintf(w, "%s -> %s\n\n", result.PreviousVersion, result.Version)
	printReleaseNotes(w, result)
	fmt.Fprintln(w, "You can see the changelog with `rest changelog`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Hooray! rest has been updated!")
}

func printReleaseNotes(w io.Writer, result selfupdate.Result) {
	if result.ReleaseNotes != "" {
		fmt.Fprintln(w, result.ReleaseNotes)
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w, "No release notes were provided for this release.")
		if result.ReleaseURL != "" {
			fmt.Fprintln(w, result.ReleaseURL)
		}
		fmt.Fprintln(w)
	}
}

func runChangelog(args []string) error {
	options, err := parseChangelogOptions(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := selfupdate.Changelog(ctx, selfupdate.Options{TargetVersion: options.version})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "rest %s\n\n", result.Version)
	printReleaseNotes(os.Stdout, result)
	return nil
}

type updateOptions struct {
	version string
	force   bool
}

type changelogOptions struct {
	version string
}

func parseChangelogOptions(args []string) (changelogOptions, error) {
	var options changelogOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version":
			i++
			if i >= len(args) {
				return changelogOptions{}, fmt.Errorf("--version requires a release tag")
			}
			options.version = args[i]
		default:
			return changelogOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return options, nil
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
	return fmt.Errorf("usage: rest init [--example sql|mongo] | rest gen | rest doctor | rest update [--version vX.Y.Z] [--force] | rest changelog [--version vX.Y.Z] | rest version")
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
