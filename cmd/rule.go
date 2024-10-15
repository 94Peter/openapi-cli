package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ory/oathkeeper/rule"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func NewOathKeeperRuleCmd() cli.Command {
	return cli.Command{
		Name:    "to-aothkeeper-rules",
		Aliases: []string{"tar"},
		Usage:   "openapi spec轉換為aothkeeper rules",
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

			return newOathKeeperRuleCmd(
				mainFile,
				outputFile,
			).Run()
		},
	}
}

func newOathKeeperRuleCmd(spec string, outputFile string) *oathKeeperRuleCmd {
	return &oathKeeperRuleCmd{
		spec:       spec,
		outputFile: outputFile,
	}
}

type oathKeeperRuleCmd struct {
	spec       string
	outputFile string
}

func (c *oathKeeperRuleCmd) Run() error {
	mainSpec, err := openapi3.NewLoader().LoadFromFile(c.spec)
	if err != nil {
		return errors.Wrap(err, "load main spec fail")
	}

	var rules []*rule.Rule

	for path, pathItem := range mainSpec.Paths.Map() {
		for method, op := range pathItem.Operations() {
			oathkeeperRules := newRules(mainSpec.Servers, method, path, op)

			// oathkeeperRule.Match = newRuleMatch(method, path)
			// for _, p := range op.Parameters {
			// 	p.Value.In = "path"
			// 	p.Value.Schema.Value.Pattern = "^[a-fA-F0-9]{24}$"
			// }
			rules = append(rules, oathkeeperRules...)
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
	fmt.Println(encoder.Encode(rules))

	fmt.Printf("API definitions written to %s\n", c.outputFile)
	return nil
}

/*
	{
	    "id": "allow-anonymous-albums",
	    "match": {
	      "url": "https://dev-sso.in-cloud.tw/api/albums",
	      "methods": ["GET"]
	    },
	    "authenticators": [
	      {
	        "handler": "bearer_token",
	        "config": {
	          "check_session_url": "http://sso-kratos:4433/sessions/whoami",
	          "preserve_path": true,
	          "preserve_query": false
	        }
	      },
	      {
	        "handler": "unauthorized"
	      }
	    ],
	    "authorizer": {
	      "handler": "allow"
	    },
	    "mutators": [
	      {
	        "handler": "header",
	        "config": {
	          "headers": {
	            "X-User": "{{ print .Subject }}"
	          }
	        }
	      }
	    ]
	  }
*/

func newRules(servers openapi3.Servers, method string, path string, op *openapi3.Operation) []*rule.Rule {
	result := make([]*rule.Rule, len(servers))
	for i, server := range servers {
		newRule := &rule.Rule{
			ID:       fmt.Sprintf("rule-%s-%d", op.OperationID, i),
			Match:    newMatch(server, method, path, op),
			Upstream: rule.Upstream{},
			Authorizer: rule.Handler{
				Handler: "allow",
				Config:  []byte("{}"),
			},
			Mutators: []rule.Handler{
				{Handler: "header", Config: []byte("{}")},
			},
			Errors: []rule.ErrorHandler{
				{Handler: "json", Config: []byte("{}")},
			},
		}
		if op.Security != nil && len(*op.Security) > 0 {
			newRule.Authenticators = []rule.Handler{
				{Handler: "bearer_token", Config: []byte(`{}`)},
				{Handler: "unauthorized"},
			}
		} else {
			newRule.Authenticators = []rule.Handler{
				{Handler: "anonymous", Config: []byte(`{"subject": "guest"}`)},
			}
		}
		result[i] = newRule
	}
	return result
}

var re = regexp.MustCompile(`\{([^}]+)\}`)

func newMatch(server *openapi3.Server, method string, url string, op *openapi3.Operation) *rule.Match {
	matches := re.FindAllStringSubmatch(url, -1)
	if len(matches) == 0 {
		return &rule.Match{
			Methods: []string{method},
			URL:     server.URL + url,
		}
	}
	for _, match := range matches {
		for _, param := range op.Parameters {
			if param.Value.In != "path" {
				continue
			}
			if param.Value.Name != match[1] {
				continue
			}
			typeName := param.Value.Schema.Value.Type.Slice()[0]
			if replace, ok := typeReplaceMap[typeName]; ok {
				url = strings.ReplaceAll(url, match[0], replace)
				continue
			}

			if typeName != "string" {
				continue
			}

			format := param.Value.Schema.Value.Format

			if replace, ok := formatReplaceMap[format]; ok {
				url = strings.ReplaceAll(url, match[0], replace)
				continue
			}
			url = strings.ReplaceAll(url, match[0], "<.*>")
		}

	}

	return &rule.Match{
		Methods: []string{method},
		URL:     server.URL + url,
	}
}

var formatReplaceMap = map[string]string{
	"objectId": "<[0-9a-fA-F]{24}>",
	"ObjectId": "<[0-9a-fA-F]{24}>",
}

var typeReplaceMap = map[string]string{
	"int":     "<[[:digit:]]+>",
	"integer": "<[[:digit:]]+>",
}
