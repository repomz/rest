package cli

import (
	"fmt"
	"runtime/debug"
)

var appVersion = "dev"

func Run(args []string, version string) error {
	appVersion = version
	return run(args)
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "gen":
		return runGen(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "list":
		return runList(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "changelog":
		return runChangelog(args[1:])
	case "version":
		fmt.Println(currentVersion())
		return nil
	default:
		return usageError()
	}
}

func usageError() error {
	return fmt.Errorf("usage: rest init [--example sql|mongo] | rest gen | rest doctor | rest list endpoints | rest update [--check] [--version vX.Y.Z] [--force] | rest changelog [--version vX.Y.Z] | rest version")
}

func currentVersion() string {
	buildVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		buildVersion = info.Main.Version
	}
	return resolveVersion(appVersion, buildVersion)
}

func resolveVersion(linkerVersion, buildVersion string) string {
	if linkerVersion != "" && linkerVersion != "dev" {
		return linkerVersion
	}
	if buildVersion != "" && buildVersion != "(devel)" {
		return buildVersion
	}
	return "dev"
}
