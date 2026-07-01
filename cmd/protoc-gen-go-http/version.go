package main

import (
	"runtime/debug"
	"strings"
)

// release is resolved from Go build metadata so `go install ...@vX.Y.Z`
// reports the tag used to build protoc-gen-go-http. Local source builds fall
// back to dev.
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
