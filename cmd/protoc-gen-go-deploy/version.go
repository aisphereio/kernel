package main

import (
	"runtime/debug"
	"strings"
)

var release = detectRelease()

func detectRelease() string {
	info, ok := debug.ReadBuildInfo()
	if ok {
		if version := strings.TrimSpace(info.Main.Version); version != "" && version != "(devel)" {
			return version
		}
	}
	return "dev"
}
