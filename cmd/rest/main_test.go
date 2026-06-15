package main

import "testing"

func TestParseConfigDir(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "default", want: "rest_config"},
		{name: "custom", args: []string{"-config", "configs/rest"}, want: "configs/rest"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseConfigDir(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("config dir = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseConfigDirRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"-config"}, {"-sqlc", "sqlc.yaml"}, {"-out", "."}} {
		if _, err := parseConfigDir(args); err == nil {
			t.Fatalf("expected error for arguments %v", args)
		}
	}
}

func TestParseOutputDir(t *testing.T) {
	got, err := parseOutputDir([]string{"-out", "project"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "project" {
		t.Fatalf("output dir = %q", got)
	}
	if _, err := parseOutputDir([]string{"-config", "rest_config"}); err == nil {
		t.Fatal("expected unknown argument error")
	}
}
