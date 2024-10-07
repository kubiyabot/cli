package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

type SwaggerSpec struct {
	Paths map[string]map[string]Operation `yaml:"paths"`
}

type Operation struct {
	Summary       string                 `yaml:"summary"`
	Description   string                 `yaml:"description"`
	OperationID   string                 `yaml:"operationId"`
	Parameters    []Parameter            `yaml:"parameters"`
	Responses     map[string]interface{} `yaml:"responses"`
	Tags          []string               `yaml:"tags"`
	HasPathParams bool                   `yaml:"-"`
}

type Parameter struct {
	Name        string `yaml:"name"`
	In          string `yaml:"in"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Type        string `yaml:"type"`
}

func main() {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Construct the path to swagger.yaml (in the same directory as the program)
	swaggerPath := filepath.Join(cwd, "swagger.yaml")

	// Read the swagger.yaml file
	data, err := ioutil.ReadFile(swaggerPath)
	if err != nil {
		fmt.Printf("Error reading swagger.yaml: %v\n", err)
		fmt.Printf("Attempted to read from: %s\n", swaggerPath)
		panic(err)
	}

	var spec SwaggerSpec
	err = yaml.Unmarshal(data, &spec)
	if err != nil {
		panic(err)
	}

	// Create a function map for the template
	funcMap := template.FuncMap{
		"contains": strings.Contains,
		"Title":    strings.Title,
	}

	// Parse the command template with the function map
	cmdTmpl, err := template.New("cmd_template.go.tmpl").Funcs(funcMap).ParseFiles("templates/cmd_template.go.tmpl")
	if err != nil {
		panic(err)
	}

	// Parse the root template
	rootTmpl, err := template.ParseFiles("templates/root.go.tmpl")
	if err != nil {
		panic(err)
	}

	// Create the cmd directory if it doesn't exist
	cmdDir := filepath.Join(cwd, "cmd")
	err = os.MkdirAll(cmdDir, 0755)
	if err != nil {
		panic(err)
	}

	// Generate root.go
	rootFile, err := os.Create(filepath.Join(cmdDir, "root.go"))
	if err != nil {
		panic(err)
	}
	defer rootFile.Close()

	err = rootTmpl.Execute(rootFile, nil)
	if err != nil {
		panic(err)
	}

	// Create a map to store commands by tag
	commandsByTag := make(map[string][]struct {
		Name          string
		Operation     Operation
		Method        string
		Path          string
		HasPathParams bool
	})

	for path, operations := range spec.Paths {
		for method, op := range operations {
			if strings.Contains(path, "action_stores") || strings.Contains(path, "workflows") {
				continue
			}
			op.HasPathParams = hasPathParams(op) // Add this line
			cmdName := formatCommandName(method, path)
			tag := strings.ToLower(op.Tags[0])
			commandsByTag[tag] = append(commandsByTag[tag], struct {
				Name          string
				Operation     Operation
				Method        string
				Path          string
				HasPathParams bool
			}{
				Name:          cmdName,
				Operation:     op,
				Method:        method,
				Path:          path,
				HasPathParams: op.HasPathParams,
			})
		}
	}

	// Generate command files for each tag
	for tag, commands := range commandsByTag {
		f, err := os.Create(filepath.Join(cmdDir, fmt.Sprintf("%s.go", tag)))
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = cmdTmpl.Execute(f, struct {
			Tag      string
			Commands []struct {
				Name          string
				Operation     Operation
				Method        string
				Path          string
				HasPathParams bool
			}
		}{
			Tag:      tag,
			Commands: commands,
		})
		if err != nil {
			panic(err)
		}
	}
}

func formatCommandName(method, path string) string {
	// Remove leading slashes and replace remaining slashes with underscores
	name := strings.TrimPrefix(path, "/")
	name = strings.ReplaceAll(name, "/", "_")

	// Remove version prefixes
	name = strings.TrimPrefix(name, "v1_")
	name = strings.TrimPrefix(name, "v2_")

	// Remove curly braces and other invalid characters
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")

	// Convert to lowercase
	name = strings.ToLower(name)

	// Use the HTTP method as the command name prefix
	switch method {
	case "get":
		if strings.HasSuffix(name, "s") {
			return "list_" + name
		}
		return "get_" + name
	case "post":
		return "create_" + name
	case "put":
		return "update_" + name
	case "delete":
		return "delete_" + name
	default:
		return method + "_" + name
	}
}

func hasPathParams(operation Operation) bool {
	for _, param := range operation.Parameters {
		if param.In == "path" {
			return true
		}
	}
	return false
}

func hasBodyParams(operation Operation) bool {
	for _, param := range operation.Parameters {
		if param.In == "body" {
			return true
		}
	}
	return false
}
