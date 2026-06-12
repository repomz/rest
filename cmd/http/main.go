package main

import (
	"fmt"
	"os"

	"github.com/repomz/viewer_backend/internal/generator"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "generate" {
		return fmt.Errorf("usage: http generate [-sqlc sqlc.yaml] [-httpgen httpgen.yaml] [-out .]")
	}

	opts := generator.Options{
		SQLCPath:    "sqlc.yaml",
		HTTPGenPath: "httpgen.yaml",
		OutDir:      ".",
	}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-sqlc":
			i++
			if i >= len(args) {
				return fmt.Errorf("-sqlc requires a path")
			}
			opts.SQLCPath = args[i]
		case "-out":
			i++
			if i >= len(args) {
				return fmt.Errorf("-out requires a path")
			}
			opts.OutDir = args[i]
		case "-httpgen":
			i++
			if i >= len(args) {
				return fmt.Errorf("-httpgen requires a path")
			}
			opts.HTTPGenPath = args[i]
		default:
			return fmt.Errorf("unknown argument %q", args[i])
		}
	}

	return generator.Generate(opts)
}
