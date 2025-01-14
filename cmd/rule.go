package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
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
			cli.StringFlag{
				Name:  "skip-tag",
				Usage: "略過rule的標籤",
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

			skipTag := c.String("skip-tag")

			return newOathKeeperRuleCmd(
				mainFile,
				outputFile,
				skipTag,
			).Run()
		},
	}
}

func newOathKeeperRuleCmd(spec, outputFile, skipTag string) *oathKeeperRuleCmd {
	return &oathKeeperRuleCmd{
		spec:       spec,
		outputFile: outputFile,
		skipTag:    skipTag,
	}
}

type oathKeeperRuleCmd struct {
	spec       string
	outputFile string
	skipTag    string
}

func (c *oathKeeperRuleCmd) Run() error {
	mainSpec, err := openapi3.NewLoader().LoadFromFile(c.spec)
	if err != nil {
		return errors.Wrap(err, "load main spec fail")
	}

	var rules []*rule.Rule

	for path, pathItem := range mainSpec.Paths.Map() {
		for method, op := range pathItem.Operations() {
			if c.skipTag != "" && containerTag(c.skipTag, op) {
				continue
			}
			oathkeeperRules := newRules(mainSpec.Servers, method, path, op)
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
	err = encoder.Encode(rules)
	if err != nil {
		log.Fatalf("Error encoding API definitions: %v", err)
	}

	fmt.Printf("API definitions written to %s\n", c.outputFile)
	return nil
}

func containerTag(tag string, op *openapi3.Operation) bool {
	for _, t := range op.Tags {
		if t == tag {
			return true
		}
	}
	return false
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
	newRule := &rule.Rule{
		ID:       fmt.Sprintf("rule-%s", op.OperationID),
		Match:    newMatch(servers, method, path, op),
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
			{Handler: "cookie_session", Config: []byte(`{}`)},
			{Handler: "bearer_token", Config: []byte(`{}`)},
			{Handler: "unauthorized"},
		}
	} else {
		newRule.Authenticators = []rule.Handler{
			{Handler: "anonymous", Config: []byte(`{"subject": "guest"}`)},
		}
	}
	return []*rule.Rule{newRule}
}

var re = regexp.MustCompile(`\{([^}]+)\}`)

const matchUrlTpl = "<https|http>://<%s>%s"

func newMatch(servers openapi3.Servers, method string, myurl string, op *openapi3.Operation) *rule.Match {
	matches := re.FindAllStringSubmatch(myurl, -1)

	hostMatch := make([]string, len(servers))
	for i, server := range servers {
		parsed, err := url.Parse(server.URL)
		if err != nil {
			panic(err)
		}
		hostMatch[i] = parsed.Host
	}
	if len(matches) == 0 {
		return &rule.Match{
			Methods: []string{method},
			URL:     fmt.Sprintf(matchUrlTpl, strings.Join(hostMatch, "|"), myurl),
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
				myurl = strings.ReplaceAll(myurl, match[0], replace)
				continue
			}

			if typeName != "string" {
				continue
			}

			format := strings.ToLower(param.Value.Schema.Value.Format)

			if replace, ok := formatReplaceMap[format]; ok {
				myurl = strings.ReplaceAll(myurl, match[0], replace)
				continue
			}

			if result, ok := joinEnum(param.Value.Schema.Value.Type, param.Value.Schema.Value.Enum); ok {
				myurl = strings.ReplaceAll(myurl, match[0], result)
				continue
			}

			myurl = strings.ReplaceAll(myurl, match[0], "<(?!.*/).*>")
		}

	}

	return &rule.Match{
		Methods: []string{method},
		URL:     fmt.Sprintf(matchUrlTpl, strings.Join(hostMatch, "|"), myurl),
	}
}

func joinEnum(typ *openapi3.Types, enum []any) (string, bool) {
	if len(enum) == 0 {
		return "", false
	}
	if typ.Includes("string") {
		var buffer bytes.Buffer
		buffer.WriteString("<")
		for i, v := range enum {
			if i > 0 {
				buffer.WriteString("|")
			}
			buffer.WriteString(v.(string))
		}
		buffer.WriteString(">")
		return buffer.String(), true
	}

	return "", false
}

var formatReplaceMap = map[string]string{
	"objectid": "<[0-9a-fA-F]{24}>",
}

var typeReplaceMap = map[string]string{
	"int":     "<[[:digit:]]+>",
	"integer": "<[[:digit:]]+>",
}
