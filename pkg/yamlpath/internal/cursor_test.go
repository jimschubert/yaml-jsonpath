package internal

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestIsRoot(t *testing.T) {
	rootNode := &yaml.Node{Value: "root"}
	rootCursor := newRootCursor(rootNode, NewYAMLCache())
	if !rootCursor.IsRoot() {
		t.Errorf("Expected root cursor to be root")
	}

	childNode := &yaml.Node{Value: "child"}
	childCursor := NewCursor(childNode, rootCursor)
	if childCursor.IsRoot() {
		t.Errorf("Expected child cursor to not be root")
	}
}

func TestAncestors(t *testing.T) {
	root := newRootCursor(&yaml.Node{Value: "root"}, NewYAMLCache())
	child := NewCursor(&yaml.Node{Value: "child"}, root)
	grandchild := NewCursor(&yaml.Node{Value: "grandchild"}, child)

	ancestors := grandchild.Ancestors()
	if len(ancestors) != 2 {
		t.Fatalf("Expected 2 ancestors, got %d", len(ancestors))
	}
	if ancestors[0] != child || ancestors[1] != root {
		t.Errorf("Ancestors order incorrect: got [%v, %v]", ancestors[0].Node().Value, ancestors[1].Node().Value)
	}
}

func TestIterAncestors(t *testing.T) {
	root := newRootCursor(&yaml.Node{Value: "root"}, NewYAMLCache())
	child := NewCursor(&yaml.Node{Value: "child"}, root)
	grandchild := NewCursor(&yaml.Node{Value: "grandchild"}, child)

	var values []string
	seq := grandchild.IterAncestors()
	seq(func(n *yaml.Node) bool {
		values = append(values, n.Value)
		return true
	})

	if len(values) != 2 {
		t.Fatalf("Expected 2 ancestor nodes, got %d", len(values))
	}
	if values[0] != "child" || values[1] != "root" {
		t.Errorf("Ancestor values order incorrect: got %v", values)
	}
}

func TestSingleRootAncestors(t *testing.T) {
	root := newRootCursor(&yaml.Node{Value: "root"}, NewYAMLCache())
	ancestors := root.Ancestors()
	if len(ancestors) != 0 {
		t.Errorf("Expected 0 ancestors for root, got %d", len(ancestors))
	}

	var count int
	seq := root.IterAncestors()
	seq(func(n *yaml.Node) bool {
		count++
		return true
	})
	if count != 0 {
		t.Errorf("Expected 0 ancestors from IterAncestors for root, got %d", count)
	}
}
func TestCursorAliases(t *testing.T) {
	yamlStr := `
foo: &testAlias bar
ref: *testAlias
`
	var rootNode yaml.Node
	if err := yaml.Unmarshal([]byte(yamlStr), &rootNode); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	cache := NewYAMLCache()
	rootCursor := newRootCursor(&rootNode, cache)

	aliases := rootCursor.Aliases()
	if aliases == nil {
		t.Fatalf("Expected aliases map, got nil")
	}
	aliasNode, ok := aliases["testAlias"]
	if !ok {
		t.Fatalf("Expected alias 'testAlias' in aliases map")
	}
	if aliasNode.Value != "bar" {
		t.Errorf("Expected alias value 'bar', got %q", aliasNode.Value)
	}
}
