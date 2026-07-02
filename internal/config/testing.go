package config

import "gopkg.in/yaml.v3"

func (t *Testing) UnmarshalYAML(node *yaml.Node) error {
	var values struct {
		HandlerTests     Enabled `yaml:"handler_tests"`
		IntegrationTests Enabled `yaml:"integration_tests"`
		LegacyTests      Enabled `yaml:"testing.T"`
		Curl             Enabled `yaml:"curl"`
	}
	if err := node.Decode(&values); err != nil {
		return err
	}
	t.HandlerTests = values.HandlerTests
	if !t.HandlerTests.Bool() {
		t.HandlerTests = values.LegacyTests
	}
	t.IntegrationTests = values.IntegrationTests
	t.Curl = values.Curl
	return nil
}
