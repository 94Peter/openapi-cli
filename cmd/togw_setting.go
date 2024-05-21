package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func NewToGwSettingCmd() cli.Command {
	return cli.Command{
		Name:    "to-gateway-setting",
		Aliases: []string{"togs"},
		Usage:   "openapi spec轉換為gateway setting",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "spec",
				Usage: "輸入主要檔案路徑",
			},
			cli.StringFlag{
				Name:  "output",
				Usage: "輸出檔案路徑",
			},
		},
		Action: func(c *cli.Context) error {
			mainFile := c.String("spec")
			if mainFile == "" {
				return errors.New("no spec file")
			}
			outputFile := c.String("output")
			if outputFile == "" {
				return errors.New("no output file")
			}
			return newToGwSettingCmd(
				mainFile,
				outputFile,
			).Run()
		},
	}
}

func newToGwSettingCmd(spec string, outputFile string) *toGwSettingCmd {
	return &toGwSettingCmd{
		spec:       spec,
		outputFile: outputFile,
	}
}

type toGwSettingCmd struct {
	spec       string
	outputFile string
}

func (c *toGwSettingCmd) Run() error {
	mainSpec, err := openapi3.NewLoader().LoadFromFile(c.spec)
	if err != nil {
		return errors.Wrap(err, "load main spec fail")
	}

	var apiDefinitions []*apiDefinition

	for path, pathItem := range mainSpec.Paths.Map() {
		for method, op := range pathItem.Operations() {
			apiDefinitions = append(apiDefinitions, newApiDefinition(method, path, op))
		}
	}
	// Create the output JSON file
	file, err := os.Create(c.outputFile)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer file.Close()

	// Encode the API definitions to JSON and write to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(map[string]any{
		"group": apiDefinitions,
	}); err != nil {
		log.Fatalf("Error encoding API definitions to JSON: %v", err)
	}

	fmt.Printf("API definitions written to %s\n", c.outputFile)
	return nil
}

type apiDefinition struct {
	Description  string   `json:"description"`
	Endpoint     string   `json:"endpoint"`
	Method       string   `json:"method"`
	Backend      string   `json:"backend"`
	Host         []string `json:"host"`
	InputQueries []string `json:"input_querys,omitempty"`
}

func (a *apiDefinition) AddInputQueryString(query string) {
	a.InputQueries = append(a.InputQueries, query)
}

func newApiDefinition(method string, path string, operation *openapi3.Operation) *apiDefinition {
	tags := make([]string, len(operation.Tags))
	for i, tag := range operation.Tags {
		tags[i] = fmt.Sprintf("#SERVICE_%s", strings.ToUpper(tag))
	}
	apiDefinition := &apiDefinition{
		Description: operation.Summary,
		Endpoint:    path,
		Method:      method,
		Backend:     path,
		Host:        tags,
	}
	for _, param := range operation.Parameters {
		if param.Value.In == "query" {
			apiDefinition.AddInputQueryString(param.Value.Name)
		}
	}
	return apiDefinition
}
