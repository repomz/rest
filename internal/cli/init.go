package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	maybePrintInitWelcome()
	maybeOfferInitUpdate()
	if err := bootstrapSQLCForInit(); err != nil {
		return err
	}
	if err := confirmInitOverlay("."); err != nil {
		return err
	}
	existingSQLC := ""
	if options.example == "" {
		var err error
		existingSQLC, err = detectExistingSQLCConfig(".")
		if err != nil {
			return err
		}
		if existingSQLC != "" && !confirmUseExistingSQLC(os.Stdin, os.Stdout, existingSQLC) {
			existingSQLC = ""
		}
	}
	switch options.example {
	case "sql":
		if err := sqlcconfig.ValidateExample("."); err != nil {
			return err
		}
	case "mongo":
	default:
		if existingSQLC == "" {
			if err := sqlcconfig.ValidateProject("."); err != nil {
				return err
			}
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
		if existingSQLC != "" {
			if err := pointRestSQLCConfigToExisting(configDir, existingSQLC); err != nil {
				return err
			}
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
		if existingSQLC != "" {
			fmt.Fprintf(os.Stdout, "Using existing SQLC config: %s\n", existingSQLC)
			return nil
		}
		if err := sqlcconfig.RemoveExample("."); err != nil {
			return err
		}
		if err := sqlcconfig.GenerateProject("."); err != nil {
			return err
		}
	}
	return nil
}

func confirmInitOverlay(root string) error {
	entries, err := visibleProjectEntries(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 || !isTerminalFile(os.Stdin) || !isTerminalFile(os.Stdout) {
		return nil
	}
	fmt.Fprintln(os.Stdout, "Existing project files were found:")
	for _, entry := range entries {
		fmt.Fprintf(os.Stdout, "- %s\n", entry)
	}
	fmt.Fprint(os.Stdout, "Continue and add rest files without deleting existing files? [Y/n]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		fmt.Fprintln(os.Stdout)
		return nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "" || answer == "y" || answer == "yes" {
		return nil
	}
	return fmt.Errorf("rest init cancelled; run it in an empty directory or continue with overlay mode")
}

func visibleProjectEntries(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		switch name {
		case ".git", ".DS_Store":
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func detectExistingSQLCConfig(root string) (string, error) {
	candidates := []string{
		"sqlc.yaml",
		"sqlc.yml",
		filepath.Join("sqlc", "sqlc.yaml"),
		filepath.Join("sqlc", "sqlc.yml"),
	}
	for _, candidate := range candidates {
		path := filepath.Join(root, candidate)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return filepath.ToSlash(candidate), nil
		}
	}
	return "", nil
}

func confirmUseExistingSQLC(r io.Reader, w io.Writer, path string) bool {
	if !isTerminalFile(os.Stdin) || !isTerminalFile(os.Stdout) {
		return true
	}
	fmt.Fprintf(w, "Existing SQLC config found: %s\n", path)
	fmt.Fprint(w, "Use it instead of generating rest_sqlc/ skeleton? [Y/n]: ")
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && len(line) == 0 {
		fmt.Fprintln(w)
		return true
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || answer == "y" || answer == "yes"
}

func pointRestSQLCConfigToExisting(configDir, sqlcPath string) error {
	path := filepath.Join(configDir, "rest_sqlc.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	relative := filepath.ToSlash(filepath.Join("..", filepath.FromSlash(sqlcPath)))
	text := strings.Replace(string(content), "  sqlc_path: ../rest_sqlc/rest_sqlc.yaml", "  sqlc_path: "+relative, 1)
	return os.WriteFile(path, []byte(text), 0o644)
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
