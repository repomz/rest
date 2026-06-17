package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	configtemplates "github.com/repomz/rest_generator/internal/config/templates"
)

func Generate(dir string) error {
	return generate(dir, false)
}

func GenerateForSQLC(dir string) error {
	return generate(dir, true)
}

func generate(dir string, enableSQLC bool) error {
	if dir == "" {
		dir = "rest_config"
	}
	type configFile struct {
		name    string
		content []byte
	}
	var files []configFile
	err := fs.WalkDir(configtemplates.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		content, err := configtemplates.Files.ReadFile(path)
		if err != nil {
			return err
		}
		if enableSQLC && filepath.Base(path) == "sqlc_rest.yaml" {
			content = []byte(strings.Replace(string(content), "  enable: disable", "  enable: enable", 1))
		}
		files = append(files, configFile{name: filepath.Base(path), content: content})
		return nil
	})
	if err != nil {
		return err
	}
	for _, file := range files {
		target := filepath.Join(dir, file.name)
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("config file already exists: %s", target)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), file.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
