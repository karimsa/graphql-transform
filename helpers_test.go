package main

import (
	"testing"
)

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSplitStringByCase(t *testing.T) {
	for _, testCase := range []struct{
		input string
		expected []string
	}{
		// One word examples
		{"hello", []string{"hello"}},
		// Two word examples
		{"helloWorld", []string{"hello", "world"}},
		{"HelloWorld", []string{"hello", "world"}},
		{"hello_world", []string{"hello", "world"}},
		{"Hello_World", []string{"hello", "world"}},
		// With acronyms
		{"helloHTTP", []string{"hello", "h", "t", "t", "p"}},
		{"HelloHTTP", []string{"hello", "h", "t", "t", "p"}},
		{"hello_http", []string{"hello", "http"}},
	} {
		if actual := splitStringByCase(testCase.input); !stringSliceEqual(actual, testCase.expected) {
			t.Errorf("Expected %s to split to %v, got %v", testCase.input, testCase.expected, actual)
		}
	}
}

func TestCamelCase(t *testing.T) {
	for _, testCase := range []struct{ input, expected string }{
		// One word examples
		{"hello", "hello"},
		// Two word examples
		{"helloWorld", "helloWorld"},
		{"HelloWorld", "helloWorld"},
		{"hello_world", "helloWorld"},
		{"Hello_World", "helloWorld"},
		// With acronyms
		{"helloHTTP", "helloHTTP"},
		{"HelloHTTP", "helloHTTP"},
		{"hello_http", "helloHttp"},
	} {
		if actual := camelCase(testCase.input); actual != testCase.expected {
			t.Errorf("Expected %s to camelCase to %s, got %s", testCase.input, testCase.expected, actual)
		}
	}
}

func TestPascalCase(t *testing.T) {
	for _, testCase := range []struct{ input, expected string }{
		// One word examples
		{"hello", "Hello"},
		// Two word examples
		{"helloWorld", "HelloWorld"},
		{"HelloWorld", "HelloWorld"},
		{"hello_world", "HelloWorld"},
		{"Hello_World", "HelloWorld"},
		// With acronyms
		{"helloHTTP", "HelloHTTP"},
		{"HelloHTTP", "HelloHTTP"},
		{"hello_http", "HelloHttp"},
	} {
		if actual := pascalCase(testCase.input); actual != testCase.expected {
			t.Errorf("Expected %s to pascalCase to %s, got %s", testCase.input, testCase.expected, actual)
		}
	}
}
