package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/repomz/rest/internal/selfupdate"
)

type changelogOptions struct {
	version string
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
