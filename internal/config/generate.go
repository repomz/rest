package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	configtemplates "github.com/repomz/rest/internal/config/templates"
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

func GenerateForMongoExample(dir string) error {
	return generate(dir, configModeMongoExample)
}

type configMode int

const (
	configModeDefault configMode = iota
	configModeSQLC
	configModeExample
	configModeMongoExample
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
		target := filepath.Join(dir, file.name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, file.content, 0o644); err != nil {
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
		if filepath.Base(path) == "auth_rest.yaml" {
			return nil
		}
		content, err := configtemplates.Files.ReadFile(path)
		if err != nil {
			return err
		}
		if mode != configModeDefault && filepath.Base(path) == "rest_sqlc.yaml" {
			content = []byte(strings.Replace(string(content), "  enable: disable", "  enable: enable", 1))
			if mode == configModeExample {
				content = []byte(strings.Replace(string(content), "  sqlc_path: ../rest_sqlc/rest_sqlc.yaml", "  sqlc_path: ../rest_sqlc_example/rest_sqlc.yaml", 1))
			}
		}
		if mode == configModeMongoExample && filepath.Base(path) == "rest.yaml" {
			text := string(content)
			for old, replacement := range map[string]string{
				"sql: enable":                     "sql: disable",
				"auto_sqlc: enable":               "auto_sqlc: disable",
				"mongo: disable":                  "mongo: enable",
				"module: github.com/repomz/myapp": "module: github.com/repomz/mongo-example",
			} {
				text = strings.Replace(text, old, replacement, 1)
			}
			text = strings.Replace(text, "  env:\n    enabled: false", "  env:\n    enabled: true", 1)
			content = []byte(text)
		}
		files = append(files, configFile{name: filepath.ToSlash(path), content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if mode == configModeMongoExample {
		files = append(files, configFile{name: "rest_mongo/item.yaml", content: []byte(mongoExampleContract)})
	}
	return files, nil
}

const mongoExampleContract = `# ==============================================================================
# REST: MONGODB ITEM EXAMPLE
# ==============================================================================
version: "0.1.0"

models:
  - name: Item
    collection: items
    timestamps: true
    fields:
      - name: id
        type: object_id
        bson: _id
        json: id
        primary: true
        generated: true

      - name: title
        type: string
        required: true

      - name: description
        type: string

      - name: status
        type: string
        required: true
        default: draft
        enum: [draft, published, archived]

      - name: tags
        type: "[]string"
        default: []
`
