package cmd

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

func NewMergeCmd() cli.Command {
	return cli.Command{
		Name:    "merge-spec",
		Aliases: []string{"ms"},
		Usage:   "合併多個openapi spec",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "main",
				Usage: "輸入主要檔案路徑",
			},
			cli.StringFlag{
				Name:  "mergeDir",
				Usage: "輸入被合併檔案路徑",
			},
			cli.StringFlag{
				Name:  "output",
				Usage: "輸出檔案路徑",
			},
			cli.StringFlag{
				Name:  "version-replace",
				Usage: "替換版本號",
			},
		},
		Action: func(c *cli.Context) error {
			mergeDir := c.String("mergeDir")
			dirEntries, err := os.ReadDir(mergeDir)
			if err != nil {
				return err
			}
			var mergeFiles []string
			for _, entry := range dirEntries {
				if entry.IsDir() {
					continue
				}
				mergeFiles = append(mergeFiles, mergeDir+entry.Name())
			}

			if len(mergeFiles) == 0 {
				return errors.New("no merge file")
			}
			mainFile := c.String("main")
			if mainFile == "" {
				return errors.New("no main file")
			}
			outputFile := c.String("output")
			if outputFile == "" {
				return errors.New("no output file")
			}
			return newMergeCmd(
				mainFile,
				mergeFiles,
				outputFile,
				c.String("version-replace"),
			).Run()
		},
	}
}

func newMergeCmd(mainFile string, mergeFiles []string, outputFile string, replaceVers string) *mergeCmd {
	return &mergeCmd{
		mainFile:    mainFile,
		mergeFiles:  mergeFiles,
		outputFile:  outputFile,
		replaceVers: replaceVers,
	}
}

type mergeCmd struct {
	mainFile    string
	mergeFiles  []string
	outputFile  string
	replaceVers string
}

func (c *mergeCmd) Run() error {
	mainSpec, err := openapi3.NewLoader().LoadFromFile(c.mainFile)
	if err != nil {
		return errors.Wrap(err, "load main spec fail")
	}
	tool := newMergeTool(mainSpec, c.replaceVers)
	for _, file := range c.mergeFiles {
		spec2, err := openapi3.NewLoader().LoadFromFile(file)
		if err != nil {
			return errors.Wrap(err, "load merge spec fail:"+file)
		}
		err = tool.Merge(spec2)
		if err != nil {
			return errors.Wrap(err, "merge spec fail")
		}
	}
	return tool.OuputYaml(c.outputFile)
}

func newMergeTool(maindoc *openapi3.T, replaceVers string) *mergeTool {
	return &mergeTool{
		doc:            maindoc,
		replaceVersion: replaceVers,
	}
}

type mergeTool struct {
	doc            *openapi3.T
	replaceVersion string
}

func (mt *mergeTool) OuputYaml(file string) error {
	type doc struct {
		Openapi    string               `yaml:"openapi"`
		Info       *openapi3.Info       `yaml:"info"`
		Servers    openapi3.Servers     `yaml:"servers,omitempty"`
		Tags       []*openapi3.Tag      `yaml:"tags,omitempty"`
		Paths      *openapi3.Paths      `yaml:"paths"`
		Components *openapi3.Components `yaml:"components,omitempty"`
	}
	mydoc := doc{
		Openapi:    mt.doc.OpenAPI,
		Info:       mt.doc.Info,
		Servers:    mt.doc.Servers,
		Tags:       mt.doc.Tags,
		Paths:      mt.doc.Paths,
		Components: mt.doc.Components,
	}
	data, err := yaml.Marshal(mydoc)
	if err != nil {
		return err
	}
	// output data to file
	return os.WriteFile(file, data, 0644)
}

func (m *mergeTool) Merge(mergeDoc *openapi3.T) error {
	m.doc.Tags = append(m.doc.Tags, &openapi3.Tag{
		Name:        mergeDoc.Info.Title,
		Description: mergeDoc.Info.Description,
	})
	if len(mergeDoc.Servers) == 0 {
		return errors.New(mergeDoc.Info.Title + " has no server")
	}
	url := mergeDoc.Servers[0].URL
	desc := mergeDoc.Servers[0].Description
	// merge all mergeDoc to mainDoc
	for k, v := range mergeDoc.Paths.Map() {
		for method, o := range v.Operations() {
			if o.Security != nil && len(*o.Security) != 0 {
				requirements := openapi3.NewSecurityRequirements()
				for k := range m.doc.Components.SecuritySchemes {
					requirements.With(openapi3.NewSecurityRequirement().Authenticate(k))
				}
				o.Security = requirements
			}

			if m.replaceVersion != "" {
				o.Summary = o.Summary + fmt.Sprintf("(對應 %s %s)", method, k)
				k = versionReplaceReg.ReplaceAllString(k, "/"+m.replaceVersion)
			}
			o.ExternalDocs = &openapi3.ExternalDocs{Description: desc, URL: url}
			m.doc.AddOperation(k, method, o)
		}

		if mergeDoc.Components == nil {
			continue
		}
		for k, v := range mergeDoc.Components.Schemas {
			if m.doc.Components.Schemas == nil {
				m.doc.Components.Schemas = make(openapi3.Schemas)
			}
			m.doc.Components.Schemas[k] = v
		}
		for k, v := range mergeDoc.Components.Examples {
			if m.doc.Components.Examples == nil {
				m.doc.Components.Examples = make(openapi3.Examples)
			}
			m.doc.Components.Examples[k] = v
		}
	}
	return nil
}
