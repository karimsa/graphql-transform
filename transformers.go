package main

import (
	"fmt"
	"strings"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
)

type FieldArgument struct {
	Name  string
	Value string
}

type GraphqlField struct {
	IsSpread   bool
	Name       string
	SourceType string
	Arguments  []FieldArgument
	SubFields  []GraphqlField
}

type Fragment struct {
	Name                 string
	SourceType           string
	Fields               []GraphqlField
	FragmentDependencies []string
}

type Variable struct {
	Name string
	Type string
}

type Operation struct {
	Name                  string
	Variables             []Variable
	Fields                []GraphqlField
	InlineFragmentSpreads []string
}

type TemplateData struct {
	Fragments []Fragment
	Queries   []Operation
	Mutations []Operation
}

func transformFieldArgumentValue(node ast.Value) (string, error) {
	switch node.GetKind() {
	case "Variable":
		return fmt.Sprintf("$%s", node.GetValue().(*ast.Name).Value), nil

	case "ListValue":
		list := node.GetValue().([]ast.Value)
		values := make([]string, 0, len(list))
		for _, item := range list {
			val, err := transformFieldArgumentValue(item)
			if err != nil {
				return "", err
			}
			values = append(values, val)
		}
		return fmt.Sprintf("[%s]", strings.Join(values, ", ")), nil

	case "EnumValue":
		fallthrough
	case "BooleanValue":
		fallthrough
	case "IntValue":
		fallthrough
	case "FloatValue":
		fallthrough
	case "StringValue":
		switch val := node.GetValue().(type) {
		case string:
			return val, nil
		case bool:
			if val {
				return "true", nil
			}
			return "fales", nil
		case interface{ String() string }:
			return val.String(), nil
		}
		return "", fmt.Errorf("invalid %s: %#v", node.GetKind(), node.GetValue())

	case "ObjectValue":
		obj := node.GetValue().([]*ast.ObjectField)
		values := make([]string, 0, len(obj))
		for _, item := range obj {
			val, err := transformFieldArgumentValue(item.Value)
			if err != nil {
				return "", err
			}
			values = append(values, fmt.Sprintf("%s: %s", item.Name.Value, val))
		}
		return fmt.Sprintf("{%s}", strings.Join(values, ", ")), nil

	default:
		return "", fmt.Errorf("Unknown argument value kind: %s (%#v)", node.GetKind(), node)
	}
}

func transformGraphqlField(def *ast.SelectionSet) ([]GraphqlField, error) {
	fields := make([]GraphqlField, 0, len(def.Selections))
	for _, selection := range def.Selections {
		if field, ok := selection.(*ast.Field); ok {
			transformedField := GraphqlField{
				Name: field.Name.Value,
			}
			if len(field.Arguments) > 0 {
				transformedField.Arguments = make([]FieldArgument, 0, len(field.Arguments))
				for _, arg := range field.Arguments {
					fieldValue, err := transformFieldArgumentValue(arg.Value)
					if err != nil {
						return nil, err
					}

					parsedFieldArg := FieldArgument{
						Name:  arg.Name.Value,
						Value: fieldValue,
					}
					transformedField.Arguments = append(transformedField.Arguments, parsedFieldArg)
				}
			}
			if field.SelectionSet != nil {
				subFields, err := transformGraphqlField(field.SelectionSet)
				if err != nil {
					return nil, err
				}
				transformedField.SubFields = subFields
			}
			fields = append(fields, transformedField)
		} else if fragmentSpread, ok := selection.(*ast.FragmentSpread); ok {
			fields = append(fields, GraphqlField{
				IsSpread: true,
				Name:     fragmentSpread.Name.Value,
			})
		} else if inlineFragment, ok := selection.(*ast.InlineFragment); ok {
			subFields, err := transformGraphqlField(inlineFragment.SelectionSet)
			if err != nil {
				return nil, err
			}

			fields = append(fields, GraphqlField{
				IsSpread:   true,
				Name:       "",
				SourceType: inlineFragment.TypeCondition.Name.Value,
				SubFields:  subFields,
			})
		} else {
			return nil, fmt.Errorf("Unknown selection kind: %t", selection)
		}
	}
	return fields, nil
}

func gatherFragmentDependencies(fields []GraphqlField) []string {
	fragmentNames := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.IsSpread && field.Name != "" {
			fragmentNames = append(fragmentNames, field.Name)
		}
		if len(field.SubFields) > 0 {
			fragmentNames = append(fragmentNames, gatherFragmentDependencies(field.SubFields)...)
		}
	}

	if len(fragmentNames) == 0 {
		return nil
	}
	return fragmentNames
}

func transformFragment(def *ast.FragmentDefinition) (Fragment, error) {
	fields, err := transformGraphqlField(def.SelectionSet)
	if err != nil {
		return Fragment{}, err
	}
	return Fragment{
		Name:                 def.Name.Value,
		SourceType:           def.TypeCondition.Name.Value,
		Fields:               fields,
		FragmentDependencies: gatherFragmentDependencies(fields),
	}, nil
}

func transformVariableType(def ast.Type) (string, error) {
	if v, ok := def.(*ast.NonNull); ok {
		subType, err := transformVariableType(v.Type)
		return fmt.Sprintf("%s!", subType), err
	}
	if v, ok := def.(*ast.List); ok {
		subType, err := transformVariableType(v.Type)
		return fmt.Sprintf("[%s]", subType), err
	}
	if v, ok := def.(*ast.Named); ok {
		return v.Name.Value, nil
	}
	return "", fmt.Errorf("Unknown type kind: %s", def.GetKind())
}

func transformOperation(def *ast.OperationDefinition) (Operation, error) {
	operation := Operation{
		Name: def.Name.Value,
	}
	if len(def.VariableDefinitions) > 0 {
		operation.Variables = make([]Variable, 0, len(def.VariableDefinitions))
		for _, varDef := range def.VariableDefinitions {
			varType, err := transformVariableType(varDef.Type)
			if err != nil {
				return operation, err
			}

			operation.Variables = append(operation.Variables, Variable{
				Name: varDef.Variable.Name.Value,
				Type: varType,
			})
		}
	}
	if def.SelectionSet != nil {
		fields, err := transformGraphqlField(def.SelectionSet)
		if err != nil {
			return operation, err
		}
		operation.Fields = fields
	}
	return operation, nil
}

func transformGraphql(templateData *TemplateData, schema string) error {
	doc, err := parser.Parse(parser.ParseParams{
		Source: schema,
		Options: parser.ParseOptions{
			NoLocation: true,
		},
	})
	if err != nil {
		return nil
	}
	if doc.Kind != "Document" {
		return fmt.Errorf("expected document to be top-level node")
	}

	for _, def := range doc.Definitions {
		switch def.GetKind() {
		case "OperationDefinition":
			switch def.(*ast.OperationDefinition).Operation {
			case "query":
				query, err := transformOperation(def.(*ast.OperationDefinition))
				if err != nil {
					return err
				}
				templateData.Queries = append(templateData.Queries, query)

			case "mutation":
				mutation, err := transformOperation(def.(*ast.OperationDefinition))
				if err != nil {
					return err
				}
				templateData.Mutations = append(templateData.Mutations, mutation)

			default:
				return fmt.Errorf("Unknown operation kind: %s", def.(*ast.OperationDefinition).Operation)
			}

		case "FragmentDefinition":
			frag, err := transformFragment(def.(*ast.FragmentDefinition))
			if err != nil {
				return err
			}
			templateData.Fragments = append(templateData.Fragments, frag)

		default:
			return fmt.Errorf("Unknown definition kind: %s", def.GetKind())
		}
	}

	return nil
}
