package appgen

import (
	"path/filepath"

	"github.com/repomz/rest/internal/config"
)

type Context struct {
	Config     config.Bundle
	ConfigDir  string
	ProjectDir string
}

func NewContext(bundle config.Bundle) Context {
	projectDir := bundle.Rest.ProjectPath
	if !filepath.IsAbs(projectDir) {
		projectDir = filepath.Clean(projectDir)
	}
	return Context{Config: bundle, ConfigDir: bundle.Dir, ProjectDir: projectDir}
}
