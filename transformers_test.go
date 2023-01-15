package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Produces a line-by-line diff of two strings
func diffStrings(left, right string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	diffLines := make([]string, 0, len(leftLines)+len(rightLines))

	// Compare each line, and add a +/- prefix to indicate whether it's in the left or right
	for i, leftLine := range leftLines {
		if i < len(rightLines) {
			rightLine := rightLines[i]
			if leftLine == rightLine {
				diffLines = append(diffLines, leftLine)
			} else {
				diffLines = append(diffLines, "- "+leftLine)
				diffLines = append(diffLines, "+ "+rightLine)
			}
		} else {
			diffLines = append(diffLines, "- "+leftLine)
		}
	}

	return strings.Join(diffLines, "\n")
}

func TestTransformGraphqlFragments(t *testing.T) {
	for _, testCase := range []struct {
		input    string
		expected TemplateData
	}{
		{
			input: `
				fragment UserFields on User {
					id
					name
				}
			`,
			expected: TemplateData{
				Fragments: []Fragment{
					{
						Name:       "UserFields",
						SourceType: "User",
						Fields: []GraphqlField{
							{
								IsSpread: false,
								Name:     "id",
							},
							{
								IsSpread: false,
								Name:     "name",
							},
						},
					},
				},
			},
		},
		{
			input: `
				fragment UserFields on User {
					id
					name

					# test spreading on a different type
					... on User {
						email
					}

					# test spreading an external fragment
					... UserFields
				}
			`,
			expected: TemplateData{
				Fragments: []Fragment{
					{
						Name:       "UserFields",
						SourceType: "User",
						Fields: []GraphqlField{
							{
								IsSpread: false,
								Name:     "id",
							},
							{
								IsSpread: false,
								Name:     "name",
							},
							{
								IsSpread:   true,
								Name:       "",
								SourceType: "User",
								SubFields: []GraphqlField{
									{
										IsSpread: false,
										Name:     "email",
									},
								},
							},
							{
								IsSpread: true,
								Name:     "UserFields",
							},
						},
						FragmentDependencies: []string{"UserFields"},
					},
				},
			},
		},
	} {
		actual := TemplateData{}
		err := transformGraphql(&actual, testCase.input)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
			return
		}

		leftJSON, err := json.MarshalIndent(testCase.expected, "", "\t")
		if err != nil {
			t.Errorf("Unexpected error parsing testCase.expected: %s", err)
			return
		}
		rightJSON, err := json.MarshalIndent(actual, "", "\t")
		if err != nil {
			t.Errorf("Unexpected error parsing actual: %s", err)
			return
		}

		if string(leftJSON) != string(rightJSON) {
			t.Errorf("Failed to transform\n\t%s", diffStrings(string(leftJSON), string(rightJSON)))
		}
	}
}
