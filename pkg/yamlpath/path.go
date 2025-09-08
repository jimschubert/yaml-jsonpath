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

// newPathFromLexer constructs a new Path using lexemes parsed from the given lexer.
// It processes path tokens recursively and returns an error for invalid syntax.
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

// identity returns a sequence containing the provided cursor if its node is valid, otherwise returns an empty sequence.
func identity(c *internal.Cursor) iter.Seq[*internal.Cursor] {
	n := c.Node()
	if n.Kind == 0 {
		return lift()
	}
	return lift(c)
}

// lift resolves and normalizes the provided cursors by evaluating aliases and merge keys, returning a sequence of results.
func lift(cursors ...*internal.Cursor) iter.Seq[*internal.Cursor] {
	resolved := make([]*internal.Cursor, 0, len(cursors))

	for _, cur := range cursors {
		if cur == nil || cur.Node() == nil {
			continue
		}

		// Repeat resolution until no further alias/merge redirection occurs.
		// Limit to a reasonable number of iterations to avoid infinite loops.
		const maxIterations = 16
		iteration := 0
		for {
			if iteration >= maxIterations {
				break
			}
			iteration++

			node := cur.Node()
			changed := false

			// Resolve alias chains
			if node.Kind == yaml.AliasNode {
				if aliases := cur.Root().Aliases(); aliases != nil {
					if anchored, ok := aliases[node.Value]; ok && anchored != nil {
						for anchored.Kind == yaml.AliasNode {
							if next, ok := aliases[anchored.Value]; ok && next != nil {
								anchored = next
							} else {
								break
							}
						}
						cur = internal.NewCursor(anchored, cur.Parent())
						changed = true
						// continue outer loop to re-evaluate the new node
					}
				}
			}

			// Normalize merge key '<<' by selecting a mapping source (resolve alias/sequence cases)
			if !changed && node.Kind == yaml.MappingNode && len(node.Content) > 0 {
				for j := 0; j < len(node.Content); j += 2 {
					if node.Content[j].Value != "<<" {
						continue
					}
					mergeNode := node.Content[j+1]
					aliases := cur.Root().Aliases()

					// merge is an alias -> resolve via alias map
					if mergeNode.Kind == yaml.AliasNode && aliases != nil {
						if anchored, ok := aliases[mergeNode.Value]; ok && anchored != nil {
							for anchored.Kind == yaml.AliasNode {
								if next, ok := aliases[anchored.Value]; ok && next != nil {
									anchored = next
								} else {
									break
								}
							}
							if anchored.Kind == yaml.MappingNode {
								cur = internal.NewCursor(anchored, cur.Parent())
								changed = true
								break
							}
						}
						continue
					}

					// merge is a sequence -> pick first mapping-like source (resolve aliases inside)
					if mergeNode.Kind == yaml.SequenceNode {
						var chosen *yaml.Node
						for _, item := range mergeNode.Content {
							if item.Kind == yaml.AliasNode && aliases != nil {
								if a, ok := aliases[item.Value]; ok && a != nil && a.Kind == yaml.MappingNode {
									chosen = a
									break
								}
							} else if item.Kind == yaml.MappingNode {
								chosen = item
								break
							}
						}
						if chosen != nil {
							cur = internal.NewCursor(chosen, cur.Parent())
							changed = true
							break
						}
						continue
					}

					// merge is directly a mapping node
					if mergeNode.Kind == yaml.MappingNode {
						cur = internal.NewCursor(mergeNode, cur.Parent())
						changed = true
						break
					}
				}
			}

			if !changed {
				break
			}
		}

		resolved = append(resolved, cur)
	}

	return slices.Values(resolved)
}

// empty returns an empty sequence of *internal.Cursor, often used as a default or placeholder value.
func empty() iter.Seq[*internal.Cursor] {
	return lift()
}

// compose combines an initial sequence of cursors with a Path to produce a flattened sequence of processed cursors.
func compose(i iter.Seq[*internal.Cursor], p *Path) iter.Seq[*internal.Cursor] {
	its := []iter.Seq[*internal.Cursor]{}
	for a := range i {
		its = append(its, p.f(a))
	}
	return flatten(its...)
}

// flatten combines multiple sequences of *internal.Cursor into a single sequence.
// It iterates over each input sequence and yields their elements in order, flattening the structure.
func flatten(i ...iter.Seq[*internal.Cursor]) iter.Seq[*internal.Cursor] {
	return func(yield func(*internal.Cursor) bool) {
		for _, next := range i {
			next(func(c *internal.Cursor) bool {
				return yield(c)
			})
		}
	}
}

// chained returns a Path by wrapping a function to transform a Cursor into a sequence of Cursors.
func chained(f func(c *internal.Cursor) iter.Seq[*internal.Cursor]) *Path {
	return &Path{f: f}
}

// propertyNameChildThen navigates to a mapping node's child with a specific key and applies the given Path on it.
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

// propertyNameBracketChildThen processes child names and updates a Path to match specific mapping node keys in YAML.
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

// propertyNameArraySubscriptThen performs a YAML path operation for a property array with a given subscript.
// It generates a sequence of cursors based on child nodes matching the subscript.
// If the subscript is "*", it applies the provided path to all matched child nodes.
// Returns a chained path for further path operations.
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

// childThen creates a Path that matches a child node with the specified name, applying the given Path to it.
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

// bracketChildNames parses a string of comma-separated, optionally quoted child names into a list of unescaped names.
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

// balanced checks if the given rune `q` is balanced (opens and closes properly) in the provided string `c`.
// Escaped quotes (preceded by a backslash) are ignored for balancing purposes.
// Returns true if the quotes are balanced, otherwise false.
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

// bracketChildThen creates a Path to match specific child nodes in a YAML mapping node, processing their corresponding values.
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

// unescape removes single backslashes unless they are escaping another backslash in the input string.
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

// allChildrenThen returns a Path that applies the given Path to all child nodes of a YAML Mapping or Sequence node.
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

// arraySubscriptThen applies a subscript and a subsequent Path to the sequence or mapping nodes in a YAML structure.
// If the node is a mapping and the subscript is "*", all values in the mapping are selected.
// For sequence nodes, the subscript is interpreted as an index or range selector.
// Returns a Path that allows chained evaluation over the selected nodes.
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

// filterThen applies a filter defined by filterLexemes to a YAML node and chains it with the provided Path p.
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

// recursiveFilterThen applies a filter and a path recursively to YAML nodes, returning a new compiled Path.
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

// mapCursors creates a slice of cursors for the given YAML nodes, linking each to the provided parent cursor.
func mapCursors(nodes []*yaml.Node, parent *internal.Cursor) []*internal.Cursor {
	cursors := make([]*internal.Cursor, len(nodes))
	for i, n := range nodes {
		cursors[i] = internal.NewCursor(n, parent)
	}
	return cursors
}

// recurse traverses a sequence of cursors recursively and yields each cursor through the provided yield function.
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
