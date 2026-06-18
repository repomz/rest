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
	return generate(dir, configModeDefault)
}

func GenerateForSQLC(dir string) error {
	return generate(dir, configModeSQLC)
}

func GenerateForExample(dir string) error {
	return generate(dir, configModeExample)
}

type configMode int

const (
	configModeDefault configMode = iota
	configModeSQLC
	configModeExample
)

func generate(dir string, mode configMode) error {
	if dir == "" {
		dir = "rest_config"
	}
	files, err := configFiles(mode)
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

type configFile struct {
	name    string
	content []byte
}

func configFiles(mode configMode) ([]configFile, error) {
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
		if mode != configModeDefault && filepath.Base(path) == "sqlc_rest.yaml" {
			content = []byte(strings.Replace(string(content), "  enable: disable", "  enable: enable", 1))
			if mode == configModeExample {
				content = []byte(strings.Replace(string(content), "  sqlc_path: ../sqlc/sqlc.yaml", "  sqlc_path: ../sqlc_example/sqlc.yaml", 1))
			}
		}
		files = append(files, configFile{name: filepath.Base(path), content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
