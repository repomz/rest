package cli

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/repomz/rest/internal/toolchain"
)

func TestMain(m *testing.M) {
	previous := ensureSQLCDependency
	ensureSQLCDependency = func(context.Context, io.Writer) (toolchain.SQLCResult, error) {
		return toolchain.SQLCResult{Path: "/test/sqlc", Version: toolchain.CompatibleSQLCVersion}, nil
	}
	code := m.Run()
	ensureSQLCDependency = previous
	os.Exit(code)
}
