package main

import (
	"github.com/shoichiimamura/kubase/cmd"
	"github.com/shoichiimamura/kubase/errors"
	"os"
)

func main() {
	command := cmd.NewCommand()
	if err := command.Execute(); err != nil {
		errors.CheckError(err)
		os.Exit(1)
	}
}
