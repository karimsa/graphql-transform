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
	"unicode"
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

	tmpl, err := template.New(target.OutputFile).Funcs(map[string]any{
		"camelCase":  camelCase,
		"pascalCase": pascalCase,
	}).Parse(string(templateStr))
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
		return len(templateData.Fragments[leftIdx].FragmentDependencies) < len(templateData.Fragments[rightIdx].FragmentDependencies)
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

func splitStringByCase(str string) []string {
	var words []string
	var word string

	for _, char := range str {
		if unicode.IsLower(char) {
			word += string(char)
		} else {
			if word != "" {
				words = append(words, word)
			}
			if unicode.IsUpper(char) {
				word = strings.ToLower(string(char))
			} else {
				word = ""
			}
		}
	}

	if word != "" {
		words = append(words, word)
	}

	return words
}

func camelCase(str string) string {
	words := splitStringByCase(str)
	for i, word := range words {
		if i == 0 {
			words[i] = strings.ToLower(word)
		} else {
			words[i] = strings.Title(strings.ToLower(word))
		}
	}
	return strings.Join(words, "")
}

func pascalCase(str string) string {
	words := splitStringByCase(str)
	for i, word := range words {
		words[i] = strings.Title(strings.ToLower(word))
	}
	return strings.Join(words, "")
}

func main() {
	confBuf, err := ioutil.ReadFile("./graphql-transform.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config from graphql-transform.json: %s\n", err.Error())
		os.Exit(1)
	}

	conf := config{}
	err = json.Unmarshal(confBuf, &conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config in graphql-transform.json: %s\n", err.Error())
		os.Exit(1)
	}

	for index, target := range conf.Targets {
		if err := buildTargets(target); err != nil {
			fmt.Fprintf(os.Stderr, "Building target [%d] using conig %v failed: %s\n", index, target, err.Error())
			os.Exit(1)
		}
	}
}
