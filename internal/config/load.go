package config

import (
	"bytes"
	"errors"
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
		if err := readYAML(filepath.Join(dir, "rest_sqlc.yaml"), &sql); err != nil {
			return Bundle{}, err
		}
		if sql.Connection.UserPassword == "" {
			sql.Connection.UserPassword = sql.Connection.LegacyUserPassword
		}
		bundle.SQL = &sql
	}
	if rest.Mongo.Bool() {
		var mongo Mongo
		if err := readYAML(filepath.Join(dir, "mongo_rest.yaml"), &mongo); err != nil {
			return Bundle{}, err
		}
		bundle.Mongo = &mongo
	}
	if rest.Auth.Bool() {
		var auth Auth
		authPath := filepath.Join(dir, "auth_rest.yaml")
		if err := readYAML(authPath, &auth); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return Bundle{}, err
			}
		} else {
			bundle.Auth = &auth
		}
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
