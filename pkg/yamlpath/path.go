/*
 * Copyright 2020 VMware, Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package yamlpath

import (
	"errors"
	"iter"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath/internal"
	"go.yaml.in/yaml/v3"
)

// Path is a compiled YAML path expression.
type Path struct {
	f            func(c *internal.Cursor) iter.Seq[*internal.Cursor]
	aliasCache   *internal.YAMLCache
	rootCacheKey string
}

// Find applies the Path to a YAML node and returns the addresses of the subnodes which match the Path.
func (p *Path) Find(node *yaml.Node) ([]*yaml.Node, error) {
	// construct a root cursor (ensures alias map on root)
	rootC := internal.NewCursor(node, nil)
	cursors := slices.Collect(p.f(rootC))
	out := make([]*yaml.Node, 0, len(cursors))
	for _, c := range cursors {
		out = append(out, c.Node())
	}
	return out, nil
}

// NewPath constructs a Path from a string expression.
func NewPath(path string) (*Path, error) {
	return newPathFromLexer(lex("Path lexer", path))
}

// NewPathWithRoot constructs a Path from a string expression, providing a root node to use for anchor/alias resolution.
func NewPathWithRoot(path string, root *yaml.Node) (*Path, error) {
	p, err := newPathFromLexer(lex("Path lexer", path))
	if err != nil {
		return nil, err
	}
	p.aliasCache = internal.NewYAMLCache()
	p.rootCacheKey, err = p.aliasCache.Store(root)
	return p, err
}

func newPathFromLexer(l *lexer) (*Path, error) {
	lx := l.nextLexeme()

	switch lx.typ {

	case lexemeError:
		return nil, errors.New(lx.val)

	case lexemeIdentity, lexemeEOF:
		return chained(identity), nil

	case lexemeRoot:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
			node := c.Node()
			if node.Kind == yaml.DocumentNode {
				node = node.Content[0]
			}
			return compose(lift(internal.NewCursor(node, c.Parent())), subPath)
		}), nil

	case lexemeRecursiveDescent:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		childName := strings.TrimPrefix(lx.val, "..")
		switch childName {
		case "*":
			return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
				return compose(recurse(c), allChildrenThen(subPath))
			}), nil
		case "":
			return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
				return compose(recurse(c), subPath)
			}), nil
		default:
			return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
				return compose(recurse(c), childThen(childName, subPath))
			}), nil
		}

	case lexemeDotChild:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		childName := strings.TrimPrefix(lx.val, ".")
		return childThen(childName, subPath), nil

	case lexemeUndottedChild:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		return childThen(lx.val, subPath), nil

	case lexemeBracketChild:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		childNames := strings.TrimSpace(lx.val)
		childNames = strings.TrimSuffix(strings.TrimPrefix(childNames, "["), "]")
		childNames = strings.TrimSpace(childNames)
		return bracketChildThen(childNames, subPath), nil

	case lexemeArraySubscript:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		subscript := strings.TrimSuffix(strings.TrimPrefix(lx.val, "["), "]")
		return arraySubscriptThen(subscript, subPath), nil

	case lexemeFilterBegin, lexemeRecursiveFilterBegin:
		var recursive bool
		if lx.typ == lexemeRecursiveFilterBegin {
			recursive = true
		}
		filterLexemes := []lexeme{}
		filterNestingLevel := 1
	f:
		for {
			lx := l.nextLexeme()
			switch lx.typ {
			case lexemeFilterBegin:
				filterNestingLevel++
			case lexemeFilterEnd:
				filterNestingLevel--
				if filterNestingLevel == 0 {
					break f
				}
			case lexemeError:
				return nil, errors.New(lx.val)
			case lexemeEOF:
				return nil, errors.New("missing end of filter")
			}
			filterLexemes = append(filterLexemes, lx)
		}
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		if recursive {
			return recursiveFilterThen(filterLexemes, subPath), nil
		}
		return filterThen(filterLexemes, subPath), nil

	case lexemePropertyName:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		childName := strings.TrimPrefix(lx.val, ".")
		childName = strings.TrimSuffix(childName, propertyName)
		return propertyNameChildThen(childName, subPath), nil

	case lexemeBracketPropertyName:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		childNames := strings.TrimSpace(lx.val)
		childNames = strings.TrimSuffix(childNames, propertyName)
		childNames = strings.TrimSuffix(strings.TrimPrefix(childNames, "["), "]")
		childNames = strings.TrimSpace(childNames)
		return propertyNameBracketChildThen(childNames, subPath), nil

	case lexemeArraySubscriptPropertyName:
		subPath, err := newPathFromLexer(l)
		if err != nil {
			return nil, err
		}
		subscript := strings.TrimSuffix(strings.TrimPrefix(lx.val, "["), "]~")
		return propertyNameArraySubscriptThen(subscript, subPath), nil

	default:
		// nothing
	}

	return nil, errors.New("invalid path syntax")
}

func identity(c *internal.Cursor) iter.Seq[*internal.Cursor] {
	n := c.Node()
	if n.Kind == 0 {
		return lift()
	}
	return lift(c)
}

func lift(cursors ...*internal.Cursor) iter.Seq[*internal.Cursor] {
	return slices.Values(cursors)
}

func empty() iter.Seq[*internal.Cursor] {
	return lift()
}

func compose(i iter.Seq[*internal.Cursor], p *Path) iter.Seq[*internal.Cursor] {
	its := []iter.Seq[*internal.Cursor]{}
	for a := range i {
		its = append(its, p.f(a))
	}
	return flatten(its...)
}

func flatten(i ...iter.Seq[*internal.Cursor]) iter.Seq[*internal.Cursor] {
	return func(yield func(*internal.Cursor) bool) {
		for _, next := range i {
			next(func(c *internal.Cursor) bool {
				return yield(c)
			})
		}
	}
}

func chained(f func(c *internal.Cursor) iter.Seq[*internal.Cursor]) *Path {
	return &Path{f: f}
}

// propertyNameChildThen: same as childThen but returns cursor for the property name node (key node).
func propertyNameChildThen(childName string, p *Path) *Path {
	childName = unescape(childName)

	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		if node.Kind != yaml.MappingNode {
			return empty()
		}
		for i, n := range node.Content {
			if i%2 == 0 && n.Value == childName {
				keyCursor := internal.NewCursor(node.Content[i], c)
				return compose(lift(keyCursor), p)
			}
		}
		return empty()
	})
}

func propertyNameBracketChildThen(childNames string, p *Path) *Path {
	unquotedChildren := bracketChildNames(childNames)

	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		if node.Kind != yaml.MappingNode {
			return empty()
		}
		its := []iter.Seq[*internal.Cursor]{}
		for _, childName := range unquotedChildren {
			for i, n := range node.Content {
				if i%2 == 0 && n.Value == childName {
					child := node.Content[i]
					childCursor := internal.NewCursor(child, c)
					its = append(its, lift(childCursor))
				}
			}
		}
		return compose(flatten(its...), p)
	})
}

func propertyNameArraySubscriptThen(subscript string, p *Path) *Path {
	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		if node.Kind == yaml.MappingNode && subscript == "*" {
			its := []iter.Seq[*internal.Cursor]{}
			for i, _ := range node.Content {
				if i%2 != 0 {
					continue // skip child values
				}
				childCursor := internal.NewCursor(node.Content[i], c)
				its = append(its, compose(lift(childCursor), p))
			}
			return flatten(its...)
		}
		return empty()
	})
}

// childThen: descend into a mapping child by name, producing child cursors.
func childThen(childName string, p *Path) *Path {
	if childName == "*" {
		return allChildrenThen(p)
	}
	childName = unescape(childName)

	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		if node.Kind != yaml.MappingNode {
			return empty()
		}
		for i, n := range node.Content {
			if i%2 == 0 && n.Value == childName {
				childNode := node.Content[i+1]
				childCursor := internal.NewCursor(childNode, c)
				return compose(lift(childCursor), p)
			}
		}
		return empty()
	})
}

func bracketChildNames(childNames string) []string {
	s := strings.Split(childNames, ",")
	// reconstitute child names with embedded commas
	children := []string{}
	accum := ""
	for _, c := range s {
		if balanced(c, '\'') && balanced(c, '"') {
			if accum != "" {
				accum += "," + c
			} else {
				children = append(children, c)
				accum = ""
			}
		} else {
			if accum == "" {
				accum = c
			} else {
				accum += "," + c
				children = append(children, accum)
				accum = ""
			}
		}
	}
	if accum != "" {
		children = append(children, accum)
	}

	unquotedChildren := []string{}
	for _, c := range children {
		c = strings.TrimSpace(c)
		if strings.HasPrefix(c, "'") {
			c = strings.TrimSuffix(strings.TrimPrefix(c, "'"), "'")
		} else {
			c = strings.TrimSuffix(strings.TrimPrefix(c, `"`), `"`)
		}
		c = unescape(c)
		unquotedChildren = append(unquotedChildren, c)
	}
	return unquotedChildren
}

func balanced(c string, q rune) bool {
	bal := true
	prev := eof
	for i := 0; i < len(c); {
		r, width := utf8.DecodeRuneInString(c[i:])
		i += width
		if r == q {
			if i > 0 && prev == '\\' {
				prev = r
				continue
			}
			bal = !bal
		}
		prev = r
	}
	return bal
}

func bracketChildThen(childNames string, p *Path) *Path {
	unquotedChildren := bracketChildNames(childNames)

	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		if node.Kind != yaml.MappingNode {
			return empty()
		}
		its := []iter.Seq[*internal.Cursor]{}
		for _, childName := range unquotedChildren {
			for i, n := range node.Content {
				if i%2 == 0 && n.Value == childName {
					childCursor := internal.NewCursor(node.Content[i+1], c)
					its = append(its, lift(childCursor))
				}
			}
		}
		return compose(flatten(its...), p)
	})
}

// keep existing helper functions unescape, bracketChildNames, balanced, etc., unchanged.
func unescape(raw string) string {
	esc := ""
	escaped := false
	for i := 0; i < len(raw); {
		rune, width := utf8.DecodeRuneInString(raw[i:])
		i += width
		if rune == '\\' {
			if escaped {
				esc += string(rune)
			}
			escaped = !escaped
			continue
		}
		escaped = false
		esc += string(rune)
	}

	return esc
}

// allChildrenThen: iterate mapping values or sequence elements, returning child cursors.
func allChildrenThen(p *Path) *Path {
	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		switch node.Kind {
		case yaml.MappingNode:
			its := []iter.Seq[*internal.Cursor]{}
			for i := 0; i < len(node.Content); i++ {
				// skip child name when i is even
				if i%2 == 0 {
					continue
				}
				child := internal.NewCursor(node.Content[i], c)
				its = append(its, compose(lift(child), p))
			}
			return flatten(its...)
		case yaml.SequenceNode:
			its := []iter.Seq[*internal.Cursor]{}
			for i := 0; i < len(node.Content); i++ {
				child := internal.NewCursor(node.Content[i], c)
				its = append(its, compose(lift(child), p))
			}
			return flatten(its...)
		default:
			return empty()
		}
	})
}

// arraySubscriptThen: slice/indices produce child cursors for sequence entries.
func arraySubscriptThen(subscript string, p *Path) *Path {
	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		if node.Kind == yaml.MappingNode && subscript == "*" {
			its := []iter.Seq[*internal.Cursor]{}
			for i := 0; i < len(node.Content); i++ {
				// mapping: select values in odd positions
				if i%2 == 0 {
					continue
				}
				child := internal.NewCursor(node.Content[i], c)
				its = append(its, compose(lift(child), p))
			}
			return flatten(its...)
		}
		if node.Kind != yaml.SequenceNode {
			return empty()
		}

		slice, err := slice(subscript, len(node.Content))
		if err != nil {
			panic(err)
		}

		its := []iter.Seq[*internal.Cursor]{}
		for _, s := range slice {
			if s >= 0 && s < len(node.Content) {
				child := internal.NewCursor(node.Content[s], c)
				its = append(its, compose(lift(child), p))
			}
		}
		return flatten(its...)
	})
}

func filterThen(filterLexemes []lexeme, p *Path) *Path {
	f := newFilter(newFilterNode(filterLexemes))
	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		its := []iter.Seq[*internal.Cursor]{}
		if node.Kind == yaml.SequenceNode {
			for _, content := range node.Content {
				childCursor := internal.NewCursor(content, c)
				if f(childCursor) {
					its = append(its, compose(lift(childCursor), p))
				}
			}
		} else {
			if f(c) {
				childCursor := internal.NewCursor(node, c)
				its = append(its, compose(lift(childCursor), p))
			}
		}
		return flatten(its...)
	})
}

func recursiveFilterThen(filterLexemes []lexeme, p *Path) *Path {
	f := newFilter(newFilterNode(filterLexemes))
	return chained(func(c *internal.Cursor) iter.Seq[*internal.Cursor] {
		node := c.Node()
		its := []iter.Seq[*internal.Cursor]{}

		if f(c) {
			childCursor := internal.NewCursor(node, c)
			its = append(its, compose(lift(childCursor), p))
		}
		return flatten(its...)
	})
}

// mapCursors converts []*yaml.Node to []*internal.Cursor with the given parent.
func mapCursors(nodes []*yaml.Node, parent *internal.Cursor) []*internal.Cursor {
	cursors := make([]*internal.Cursor, len(nodes))
	for i, n := range nodes {
		cursors[i] = internal.NewCursor(n, parent)
	}
	return cursors
}

func recurse(i ...*internal.Cursor) iter.Seq[*internal.Cursor] {
	return func(yield func(*internal.Cursor) bool) {
		for _, n := range i {
			children := mapCursors(n.Node().Content, n)
			recurse(children...)(yield)
			if !yield(n) {
				return
			}
		}
	}
}
