package appgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type generationState struct {
	Fingerprint string `json:"fingerprint"`
}

const generationContractVersion = "auth-v6"

func generationFingerprint(ctx Context) (string, error) {
	paths := []string{filepath.Join(ctx.ConfigDir, "rest.yaml")}
	if ctx.Config.Rest.SQL.Bool() {
		paths = append(paths, filepath.Join(ctx.ConfigDir, "rest_sqlc.yaml"))
	}
	if ctx.Config.Rest.Mongo.Bool() {
		paths = append(paths, filepath.Join(ctx.ConfigDir, "mongo_rest.yaml"))
		files, err := mongoContractFiles(ctx)
		if err != nil {
			return "", err
		}
		paths = append(paths, files...)
	}
	authPath := filepath.Join(ctx.ConfigDir, "auth_rest.yaml")
	if ctx.Config.Rest.Auth.Bool() {
		if _, err := os.Stat(authPath); err == nil {
			paths = append(paths, authPath)
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if ctx.Config.SQL != nil && ctx.Config.SQL.SQLC.Path != "" {
		sqlcPath := resolveSQLCPath(ctx.ConfigDir, ctx.Config.SQL.SQLC.Path)
		paths = append(paths, sqlcPath)
		inputs, err := sqlcInputFiles(sqlcPath)
		if err != nil {
			return "", err
		}
		paths = append(paths, inputs...)
	}
	sort.Strings(paths)
	hash := sha256.New()
	if _, err := io.WriteString(hash, generationContractVersion+"\x00"); err != nil {
		return "", err
	}
	for _, path := range paths {
		if _, err := io.WriteString(hash, filepath.Clean(path)+"\x00"); err != nil {
			return "", err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if _, err := hash.Write(content); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sqlcInputFiles(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		SQL []struct {
			Queries interface{} `yaml:"queries"`
			Schema  interface{} `yaml:"schema"`
		} `yaml:"sql"`
	}
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse sqlc inputs: %w", err)
	}
	if len(document.SQL) == 0 {
		return nil, nil
	}
	base := filepath.Dir(path)
	var files []string
	for _, value := range append(yamlPaths(document.SQL[0].Queries), yamlPaths(document.SQL[0].Schema)...) {
		if !filepath.IsAbs(value) {
			value = filepath.Join(base, value)
		}
		info, err := os.Stat(value)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, value)
			continue
		}
		matches, err := filepath.Glob(filepath.Join(value, "*.sql"))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	return files, nil
}

func yamlPaths(value interface{}) []string {
	switch value := value.(type) {
	case string:
		return []string{value}
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if path, ok := item.(string); ok {
				result = append(result, path)
			}
		}
		return result
	default:
		return nil
	}
}

func generationUnchanged(projectDir, fingerprint string) (bool, error) {
	content, err := os.ReadFile(generationStatePath(projectDir))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var state generationState
	if err := json.Unmarshal(content, &state); err != nil {
		return false, nil
	}
	return state.Fingerprint == fingerprint, nil
}

func saveGenerationFingerprint(projectDir, fingerprint string) error {
	path := generationStatePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(generationState{Fingerprint: fingerprint}, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}

func generationStatePath(projectDir string) string {
	return filepath.Join(projectDir, ".rest", "generation.json")
}
