package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Load(dir string) (Bundle, error) {
	restPath := filepath.Join(dir, "rest.yaml")
	var rest Rest
	if err := readYAML(restPath, &rest); err != nil {
		return Bundle{}, err
	}
	bundle := Bundle{Dir: dir, Rest: rest}
	if rest.SQL.Bool() {
		var sql SQL
		if err := readYAML(filepath.Join(dir, "sqlc_rest.yaml"), &sql); err != nil {
			return Bundle{}, err
		}
		if sql.Connection.UserPassword == "" {
			sql.Connection.UserPassword = sql.Connection.LegacyUserPassword
		}
		bundle.SQL = &sql
	}
	return bundle, nil
}

func readYAML(path string, target interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}
