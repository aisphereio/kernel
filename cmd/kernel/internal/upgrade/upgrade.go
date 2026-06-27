package upgrade

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aisphereio/kernel/cmd/kernel/internal/base"
)

// CmdUpgrade represents the upgrade command.
var CmdUpgrade = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the kernel tools",
	Long:  "Upgrade the kernel tools. Example: kernel upgrade",
	Run:   Run,
}

// Run upgrade the kernel tools.
func Run(_ *cobra.Command, _ []string) {
	err := base.GoInstall(
		"github.com/aisphereio/kernel/cmd/kernel@latest",
		"github.com/aisphereio/kernel/cmd/protoc-gen-go-http@latest",
		"github.com/aisphereio/kernel/cmd/protoc-gen-go-errors@latest",
		"google.golang.org/protobuf/cmd/protoc-gen-go@latest",
		"google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest",
		"github.com/google/gnostic/cmd/protoc-gen-openapi@latest",
	)
	if err != nil {
		fmt.Println(err)
	}
}
