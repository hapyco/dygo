package cli

import (
	"runtime/debug"
	"strings"
)

var version = "dev"
var readBuildInfo = debug.ReadBuildInfo

func currentVersion() string {
	if cliVersion := strings.TrimSpace(version); cliVersion != "" && cliVersion != "dev" {
		return cliVersion
	}
	buildInfo, ok := readBuildInfo()
	if ok {
		buildVersion := strings.TrimSpace(buildInfo.Main.Version)
		if buildVersion != "" && buildVersion != "(devel)" {
			return buildVersion
		}
	}
	return "dev"
}

func dygoVersionForNew() string {
	return currentVersion()
}
