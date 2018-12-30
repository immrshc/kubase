package main

import (
	"github.com/shoichiimamura/kubase/cmd"
	"log"
)

func main() {
	command := cmd.NewCommand()
	if err := command.Execute(); err != nil {
		log.Fatal(err)
	}
}
