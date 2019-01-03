package cmd

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "kubase",
		Run: runHelp,
	}
	command.AddCommand(NewEditCommand())
	command.AddCommand(NewDecodeCommand())
	return command
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}
