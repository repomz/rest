package sqlcconfig

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// templateFiles contains internal SQLC assets shipped inside the rest binary.
//
//go:embed templates/project templates/example
var templateFiles embed.FS

func GenerateProject(root string) error {
	return generate(root, "project")
}

func ValidateProject(root string) error {
	files, err := plannedFiles(root, "project")
	if err != nil {
		return err
	}
	return validateTargets(files)
}

func GenerateExample(root string) error {
	return generate(root, "example")
}

func generate(root, templateRoot string) error {
	files, err := plannedFiles(root, templateRoot)
	if err != nil {
		return err
	}
	if err := validateTargets(files); err != nil {
		return err
	}
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file.path, file.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

type outputFile struct {
	path    string
	content []byte
}

func plannedFiles(root, templateRoot string) ([]outputFile, error) {
	if root == "" {
		root = "."
	}
	var files []outputFile
	templateRoot = "templates/" + templateRoot
	err := fs.WalkDir(templateFiles, templateRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := templateFiles.ReadFile(path)
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(path, templateRoot+"/")
		files = append(files, outputFile{path: filepath.Join(root, filepath.FromSlash(relative)), content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func validateTargets(files []outputFile) error {
	for _, file := range files {
		if _, err := os.Stat(file.path); err == nil {
			return fmt.Errorf("SQLC file already exists: %s", file.path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
