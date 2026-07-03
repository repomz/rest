package cli

import (
	"errors"
	"strings"
)

func FormatError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	hints := errorHints(message)
	if len(hints) == 0 {
		return message
	}
	var b strings.Builder
	b.WriteString(message)
	b.WriteString("\n\nHint:\n")
	for _, hint := range hints {
		b.WriteString("- ")
		b.WriteString(hint)
		b.WriteByte('\n')
	}
	var doctorErr doctorFailedError
	if !errors.As(err, &doctorErr) {
		b.WriteString("\nRun `rest doctor` for a full project readiness check.")
	}
	return strings.TrimRight(b.String(), "\n")
}

func errorHints(message string) []string {
	lower := strings.ToLower(message)
	var hints []string
	add := func(hint string) {
		for _, existing := range hints {
			if existing == hint {
				return
			}
		}
		hints = append(hints, hint)
	}

	if strings.Contains(lower, "rest_config directory is missing") || strings.Contains(lower, "open rest_config") {
		add("Run `rest init` from the application root before `rest gen`.")
	}
	if strings.Contains(lower, "invalid yaml") || strings.Contains(lower, "duplicate key") || strings.Contains(lower, "yaml") {
		add("Fix YAML syntax, indentation, and duplicate keys in `rest_config`.")
	}
	if (strings.Contains(lower, "field") && strings.Contains(lower, "not found")) || strings.Contains(lower, "unknown field") {
		add("Remove or rename unsupported config fields; compare the file with the current generated template.")
	}
	if strings.Contains(lower, "sqlc") && (strings.Contains(lower, "executable file not found") || strings.Contains(lower, "not installed")) {
		add("Install sqlc with `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0` and make sure it is in PATH.")
	}
	if strings.Contains(lower, "sqlc config") || strings.Contains(lower, "rest_sqlc.yaml") {
		add("Check `rest_config/rest_sqlc.yaml` and paths inside `rest_config/rest_sqlc/rest_sqlc.yaml`.")
	}
	if strings.Contains(lower, "auth endpoint") && strings.Contains(lower, "public") && strings.Contains(lower, "require_auth") {
		add("In `auth_rest.yaml`, an endpoint must be either public or require auth, not both.")
	}
	if strings.Contains(lower, "auth_rest.yaml") {
		add("If auth was just enabled, run `rest gen`, configure generated endpoint policies, then run `rest gen` again.")
	}
	if strings.Contains(lower, "docker") && (strings.Contains(lower, "daemon") || strings.Contains(lower, "cannot connect") || strings.Contains(lower, "timed out")) {
		add("Start Docker Desktop or the Docker daemon, then verify with `docker info`.")
	}
	if strings.Contains(lower, "mongo") || strings.Contains(lower, "mongo_uri") || strings.Contains(lower, "mongodb") {
		add("Check MongoDB is running and set the configured Mongo URI environment variable, usually `MONGO_URI`.")
	}
	if strings.Contains(lower, "postgres") || strings.Contains(lower, "db_dsn") || strings.Contains(lower, "pq:") {
		add("Set `DB_DSN` to a valid PostgreSQL connection string before running the generated app.")
	}
	if strings.Contains(lower, "checksum") {
		add("Do not replace the binary when release checksum verification fails.")
	}
	return hints
}
