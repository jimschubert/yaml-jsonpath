package internal

import (
	"iter"

	"go.yaml.in/yaml/v3"
)

type Cursor struct {
	node           *yaml.Node
	parent         *Cursor
	root           *Cursor
	aliases        *YAMLCache // only set on root
	hashAtCreation string
}

// NewCursor constructs a child cursor with a parent.
func NewCursor(node *yaml.Node, parent *Cursor) *Cursor {
	if parent == nil {
		aliasCache := NewYAMLCache()
		c := newRootCursor(node, aliasCache)
		return c
	}

	return &Cursor{
		node:   node,
		parent: parent,
		root:   parent.root,
	}
}

// Node returns the YAML node at this cursor.
func (c *Cursor) Node() *yaml.Node {
	return c.node
}

// Parent returns the parent cursor.
func (c *Cursor) Parent() *Cursor {
	return c.parent
}

// Root returns the root cursor.
func (c *Cursor) Root() *Cursor {
	return c.root
}

// Aliases returns the alias map from the root cursor.
func (c *Cursor) Aliases() map[string]*yaml.Node {
	if c.root != nil {
		aliases, _ := c.aliases.GetAllAliases(c.hashAtCreation)
		return aliases
	}
	return nil
}

// IsRoot returns true if the cursor is at the root node (no parent).
func (c *Cursor) IsRoot() bool {
	return c.parent == nil
}

// Ancestors returns a slice of Cursors from the current node up to the root (excluding self).
func (c *Cursor) Ancestors() []*Cursor {
	var ancestors []*Cursor
	cur := c.parent
	for cur != nil {
		ancestors = append(ancestors, cur)
		cur = cur.parent
	}
	return ancestors
}

// IterAncestors yields each ancestor node up to the root as an iter.Seq[*yaml.Node].
func (c *Cursor) IterAncestors() iter.Seq[*yaml.Node] {
	return func(yield func(*yaml.Node) bool) {
		cur := c.parent
		for cur != nil {
			if !yield(cur.node) {
				return
			}
			cur = cur.parent
		}
	}
}

// newRootCursor constructs a root cursor with an alias map.
func newRootCursor(node *yaml.Node, aliases *YAMLCache) *Cursor {
	c := &Cursor{
		node:    node,
		parent:  nil,
		aliases: aliases,
	}
	c.root = c
	if aliases != nil {
		c.hashAtCreation, _ = aliases.Store(node)
	}

	return c
}
