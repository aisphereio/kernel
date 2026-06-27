package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/aisphereio/kernel/cmd/kernel/internal/change"
	"github.com/aisphereio/kernel/cmd/kernel/internal/project"
	"github.com/aisphereio/kernel/cmd/kernel/internal/proto"
	"github.com/aisphereio/kernel/cmd/kernel/internal/run"
	"github.com/aisphereio/kernel/cmd/kernel/internal/upgrade"
)

var rootCmd = &cobra.Command{
	Use:     "kernel",
	Short:   "Aisphere Kernel: AI-native application kernel for Go services.",
	Long:    `Aisphere Kernel: AI-native application kernel for Go services.`,
	Version: release,
}

func init() {
	rootCmd.AddCommand(project.CmdNew)
	rootCmd.AddCommand(proto.CmdProto)
	rootCmd.AddCommand(upgrade.CmdUpgrade)
	rootCmd.AddCommand(change.CmdChange)
	rootCmd.AddCommand(run.CmdRun)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Failed to execute command: %v", err)
	}
}
