package dbx_test

import (
	"strings"
	"testing"
)

func skipIfContainerProviderUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"get provider",
		"rootless docker is not supported",
		"cannot connect to the docker daemon",
		"docker is not available",
		"no docker host",
	} {
		if strings.Contains(msg, marker) {
			t.Skipf("skipping integration test because testcontainers provider is unavailable: %v", err)
		}
	}
}
