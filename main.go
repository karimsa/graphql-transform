package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type configTarget struct {
	SchemaFile   []string `json:"schema"`
	TemplateFile string   `json:"template"`
	OutputFile   string   `json:"output"`
}

type config struct {
	Targets []configTarget `json:"targets"`
}

func buildTarget(schemaFile string, tmpl *template.Template, output io.Writer) error {
	schema, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return err
	}

	tmplData, err := transformGraphql(string(schema))
	if err != nil {
		return err
	}

	return tmpl.Execute(output, tmplData)
}

func buildTargets(target configTarget) error {
	buildStart := time.Now()

	templateStr, err := ioutil.ReadFile(target.TemplateFile)
	if err != nil {
		return err
	}

	tmpl, err := template.New(target.OutputFile).Parse(string(templateStr))
	if err != nil {
		return err
	}

	fd, err := os.OpenFile(target.OutputFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()
	fmt.Printf("\nBuilding %s using %s\n", target.OutputFile, target.TemplateFile)

	for _, schemaFileGlob := range target.SchemaFile {
		if strings.HasPrefix(schemaFileGlob, "./") {
			schemaFileGlob = schemaFileGlob[2:]
		}

		absGlob, err := filepath.Abs(schemaFileGlob)
		if err != nil {
			return err
		}

		walkPath := filepath.Dir(absGlob)
		for strings.Contains(walkPath, "*") {
			walkPath = filepath.Dir(walkPath)
		}

		err = fs.WalkDir(os.DirFS(walkPath), ".", func(schemaFilePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			schemaFilePath = filepath.Join(walkPath, schemaFilePath)
			if matched, err := path.Match(absGlob, schemaFilePath); matched {
				fmt.Printf(" > adding: %s\n", schemaFilePath)
				err := buildTarget(schemaFilePath, tmpl, fd)
				if err != nil {
					return err
				}
			} else {
				fmt.Printf("fuck: %s - %s (%#v)\n", absGlob, schemaFilePath, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	fmt.Printf("Built in %s\n\n", time.Since(buildStart).Truncate(time.Millisecond).String())
	return nil
}

func main() {
	confBuf, err := ioutil.ReadFile("./graphql-transform.json")
	if err != nil {
		panic(err)
	}

	conf := config{}
	err = json.Unmarshal(confBuf, &conf)
	if err != nil {
		panic(err)
	}

	for _, target := range conf.Targets {
		if err := buildTargets(target); err != nil {
			panic(err)
		}
	}
}
