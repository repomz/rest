package cli

import (
	"fmt"

	"github.com/repomz/rest/internal/appgen"
	"github.com/repomz/rest/internal/config"
)

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
