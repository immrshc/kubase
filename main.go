package main

import (
	"github.com/shoichiimamura/kubase/cmd"
	"github.com/shoichiimamura/kubase/util"
	"os"
)

func main() {
	command := cmd.NewCommand()
	if err := command.Execute(); err != nil {
		util.ErrorCheck(err)
		os.Exit(1)
	}
}
