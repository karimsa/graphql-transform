package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
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

	fmt.Printf("\nBuilding %s using %s\n", target.OutputFile, target.TemplateFile)

	templateData := TemplateData{
		Fragments: make([]Fragment, 0),
		Queries:   make([]Operation, 0),
		Mutations: make([]Operation, 0),
	}
	visitedFiles := make(map[string]bool, 100)

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
			if _, ok := visitedFiles[schemaFilePath]; ok {
				return nil
			}

			if matched, _ := path.Match(absGlob, schemaFilePath); matched {
				fmt.Printf(" > adding: %s\n", schemaFilePath)

				schema, err := ioutil.ReadFile(schemaFilePath)
				if err != nil {
					return err
				}

				err = transformGraphql(&templateData, string(schema))
				if err != nil {
					return err
				}

				visitedFiles[schemaFilePath] = true
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	sort.SliceStable(templateData.Fragments, func(leftIdx, rightIdx int) bool {
		return len(templateData.Fragments[leftIdx].DependentFragments) < len(templateData.Fragments[rightIdx].DependentFragments)
	})

	fd, err := os.Create(target.OutputFile)
	if err != nil {
		return err
	}
	defer fd.Close()

	err = tmpl.Execute(fd, templateData)
	if err != nil {
		return err
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
