package main

import (
	"flag"
	"fmt"
	"openapi-cli/cmd"
	"os"
	"time"

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

var (
	v         = flag.Bool("v", false, "version")
	Version   = "1.0.0"
	BuildTime = time.Now().Local().GoString()
)

func main() {
	flag.Parse()
	if *v {
		fmt.Println("Version: " + Version)
		fmt.Println("Build Time: " + BuildTime)
		return
	}

	app := cli.NewApp()
	app.Commands = []cli.Command{
		cmd.NewMergeCmd(),
		cmd.NewToGwSettingCmd(),
	}
	err := app.Run(os.Args)

	if err != nil {
		panic(err)
	}
}
