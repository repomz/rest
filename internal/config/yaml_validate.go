package config

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func ValidateYAMLTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !isYAMLFile(path) {
			return nil
		}
		return ValidateYAMLFile(path)
	})
}

func ValidateYAMLFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read YAML %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for document := 1; ; document++ {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("invalid YAML %s: %w", path, err)
		}
		if node.Kind == 0 {
			return nil
		}
		if err := validateYAMLNode(path, document, "$", &node); err != nil {
			return err
		}
	}
}

func validateYAMLNode(path string, document int, location string, node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := validateYAMLNode(path, document, location, child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := map[string]*yaml.Node{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			keyName := key.Value
			if previous := seen[keyName]; previous != nil {
				return fmt.Errorf("invalid YAML %s: duplicate key %q at %s in document %d (previous line %d, duplicate line %d)", path, keyName, location, document, previous.Line, key.Line)
			}
			seen[keyName] = key
			nextLocation := location + "." + keyName
			if err := validateYAMLNode(path, document, nextLocation, value); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if err := validateYAMLNode(path, document, fmt.Sprintf("%s[%d]", location, i), child); err != nil {
				return err
			}
		}
	}
	return nil
}

func isYAMLFile(path string) bool {
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}
