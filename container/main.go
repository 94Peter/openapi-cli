package main

import (
	"openapi-cli/cmd"
	"os"

	"github.com/urfave/cli"
)

// APIDefinition represents the structure of the API definition in JSON
type APIDefinition struct {
	Description string   `json:"description"`
	Endpoint    string   `json:"endpoint"`
	Method      string   `json:"method"`
	Backend     string   `json:"backend"`
	Host        []string `json:"host"`
}

func main() {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		cmd.NewMergeCmd(),
		cmd.NewToGwSettingCmd(),
		cmd.NewOathKeeperRuleCmd(),
	}
	err := app.Run(os.Args)

	if err != nil {
		panic(err)
	}
}
