package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/repomz/rest/internal/config"
	"github.com/repomz/rest/internal/selfupdate"
	"github.com/repomz/rest/internal/sqlcconfig"
)

var (
	initUpdateCheck          = selfupdate.Check
	initUpdateInstall        = selfupdate.Update
	initUpdateCheckTimeout   = 2 * time.Second
	initUpdateInstallTimeout = 2 * time.Minute
)

type initOptions struct {
	example string
}

func runInit(args []string) error {
	options, err := parseInitOptions(args)
	if err != nil {
		return err
	}
	maybeOfferInitUpdate()
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

func maybeOfferInitUpdate() {
	current := currentVersion()
	if current == "dev" || !isTerminalFile(os.Stdin) || !isTerminalFile(os.Stdout) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), initUpdateCheckTimeout)
	defer cancel()
	result, err := initUpdateCheck(ctx, selfupdate.Options{
		CurrentVersion: current,
		Stdout:         io.Discard,
	})
	if err != nil || !result.Available {
		return
	}
	if !confirmInitUpdate(os.Stdin, os.Stdout, result) {
		fmt.Fprintln(os.Stdout, "Continuing rest init.")
		fmt.Fprintln(os.Stdout)
		return
	}
	updateCtx, updateCancel := context.WithTimeout(context.Background(), initUpdateInstallTimeout)
	defer updateCancel()
	updated, err := initUpdateInstall(updateCtx, selfupdate.Options{
		CurrentVersion: current,
		Stdout:         os.Stdout,
	})
	if err != nil {
		fmt.Fprintf(os.Stdout, "Update failed: %v\n", err)
		fmt.Fprintln(os.Stdout, "Continuing rest init.")
		fmt.Fprintln(os.Stdout)
		return
	}
	if updated.Updated {
		printUpdateResult(os.Stdout, updated)
	} else {
		fmt.Fprintf(os.Stdout, "rest is already up to date (%s)\n", updated.Version)
	}
	fmt.Fprintln(os.Stdout, "Continuing rest init.")
	fmt.Fprintln(os.Stdout)
}

func confirmInitUpdate(r io.Reader, w io.Writer, result selfupdate.Result) bool {
	fmt.Fprintf(w, "A newer rest version is available: %s", result.Version)
	if result.PreviousVersion != "" {
		fmt.Fprintf(w, " (current: %s)", result.PreviousVersion)
	}
	fmt.Fprintln(w)
	fmt.Fprint(w, "Update before initializing the project? [y/N]: ")
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && len(line) == 0 {
		fmt.Fprintln(w)
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func isTerminalFile(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
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
