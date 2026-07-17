package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/repomz/rest/internal/toolchain"
)

var ensureSQLCDependency = toolchain.EnsureSQLC

func bootstrapSQLC(timeout time.Duration, output io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if _, err := ensureSQLCDependency(ctx, output); err != nil {
		return fmt.Errorf("prepare compatible sqlc: %w; you can retry manually with `%s`", err, toolchain.SQLCInstallCommand())
	}
	return nil
}

func bootstrapSQLCForInit() error {
	return bootstrapSQLC(5*time.Minute, os.Stdout)
}
