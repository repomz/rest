package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/repomz/rest/internal/selfupdate"
)

type updateOptions struct {
	version string
	force   bool
	check   bool
}

func runUpdate(args []string) error {
	options, err := parseUpdateOptions(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if options.check {
		result, err := selfupdate.Check(ctx, selfupdate.Options{
			CurrentVersion: currentVersion(),
			TargetVersion:  options.version,
			Force:          options.force,
			Stdout:         os.Stdout,
		})
		if err != nil {
			return err
		}
		printUpdateCheckResult(os.Stdout, result)
		return nil
	}
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
		return bootstrapSQLC(5*time.Minute, os.Stdout)
	}
	printUpdateResult(os.Stdout, result)
	return bootstrapSQLC(5*time.Minute, os.Stdout)
}

func printUpdateCheckResult(w io.Writer, result selfupdate.Result) {
	if result.Available {
		fmt.Fprintf(w, "New rest version available: %s", result.Version)
		if result.PreviousVersion != "" {
			fmt.Fprintf(w, " (current: %s)", result.PreviousVersion)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Run `rest update` to install it.")
		return
	}
	fmt.Fprintf(w, "rest is already up to date (%s)\n", result.Version)
}

func printUpdateResult(w io.Writer, result selfupdate.Result) {
	fmt.Fprintln(w, "Updating rest")
	fmt.Fprintf(w, "%s -> %s\n\n", result.PreviousVersion, result.Version)
	if result.Checksum != "" {
		fmt.Fprintf(w, "Verified SHA-256: %s\n", result.Checksum)
		fmt.Fprintln(w)
	}
	printReleaseNotes(w, result)
	fmt.Fprintln(w, "You can see the changelog with `rest changelog`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Hooray! rest has been updated!")
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
		case "--check":
			options.check = true
		default:
			return updateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return options, nil
}
