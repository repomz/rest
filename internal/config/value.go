package config

import (
	"fmt"
	"strings"
)

type Enabled bool

func (e *Enabled) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value interface{}
	if err := unmarshal(&value); err != nil {
		return err
	}
	switch value := value.(type) {
	case bool:
		*e = Enabled(value)
		return nil
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "enable", "enabled", "on", "yes":
			*e = true
			return nil
		case "false", "disable", "disabled", "off", "no", "":
			*e = false
			return nil
		}
	}
	return fmt.Errorf("unsupported enabled value %v", value)
}

func (e Enabled) Bool() bool {
	return bool(e)
}
