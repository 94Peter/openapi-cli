package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"

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
			cli.StringFlag{
				Name:  "version-replace",
				Usage: "替換版本號",
			},
			cli.StringFlag{
				Name:  "no-redirect-tag",
				Usage: "替換版本號",
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
			noRedirectTag := c.String("no-redirect-tag")

			versionReplace := c.String("version-replace")
			return newToGwSettingCmd(
				mainFile,
				outputFile,
				versionReplace,
				noRedirectTag,
			).Run()
		},
	}
}

func newToGwSettingCmd(spec string, outputFile string, versionreplace string, noRedirectTag string) *toGwSettingCmd {
	return &toGwSettingCmd{
		spec:           spec,
		outputFile:     outputFile,
		versionReplace: versionreplace,
		noRedirectTag:  noRedirectTag,
	}
}

type toGwSettingCmd struct {
	spec           string
	outputFile     string
	versionReplace string
	noRedirectTag  string
}

func (c *toGwSettingCmd) Run() error {
	mainSpec, err := openapi3.NewLoader().LoadFromFile(c.spec)
	if err != nil {
		return errors.Wrap(err, "load main spec fail")
	}

	var apiDefinitions []*apiDefinition
	var noRedirect bool
	for path, pathItem := range mainSpec.Paths.Map() {
		for method, op := range pathItem.Operations() {
			noRedirect = false
			if c.noRedirectTag != "" {
				for _, tag := range op.Tags {
					if tag == c.noRedirectTag {
						noRedirect = true
						break
					}
				}
			}
			apiDefinitions = append(apiDefinitions,
				newApiDefinition(method, path, op, c.versionReplace, noRedirect))
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
	NoRedirect   bool     `json:"no_redirect,omitempty"`
}

func (a *apiDefinition) AddInputQueryString(query string) {
	a.InputQueries = append(a.InputQueries, query)
}

var versionReplaceReg = regexp.MustCompile(`/v[0-9]+`)

func newApiDefinition(method string, path string, operation *openapi3.Operation, replaceVersion string, noRedirect bool) *apiDefinition {
	parsedUrl, err := url.Parse(operation.ExternalDocs.URL)
	if err != nil {
		panic(err)
	}
	prepath := parsedUrl.Path
	parsedUrl.Path = ""
	host := []string{parsedUrl.String()}
	var endpoint string
	if replaceVersion != "" {
		endpoint = versionReplaceReg.ReplaceAllString(path, "/"+replaceVersion)
	} else {
		endpoint = path
	}
	if prepath != "" {
		path = prepath + path
	}
	apiDefinition := &apiDefinition{
		Description: operation.Summary,
		Endpoint:    endpoint,
		Method:      method,
		Backend:     path,
		Host:        host,
		NoRedirect:  noRedirect,
	}
	for _, param := range operation.Parameters {
		if param.Value.In == "query" {
			apiDefinition.AddInputQueryString(param.Value.Name)
		}
	}
	return apiDefinition
}
