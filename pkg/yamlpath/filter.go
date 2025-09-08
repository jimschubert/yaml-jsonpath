package yamlpath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath/internal"
	"go.yaml.in/yaml/v3"
)

// filter is a function type that determines whether a given *internal.Cursor satisfies a specific condition.
type filter func(c *internal.Cursor) bool

// newFilter creates and returns a filter based on the given filterNode parse tree.
// It evaluates various filter conditions like comparison, logical operations, and path existence.
// If the node is nil or unhandled, it defaults to a filter that always returns false.
func newFilter(n *filterNode) filter {
	if n == nil {
		return never
	}

	switch n.lexeme.typ {
	case lexemeFilterAt, lexemeRoot:
		path := pathFilterScanner(n)
		return func(c *internal.Cursor) bool {
			return len(path(c)) > 0
		}

	case lexemeFilterEquality, lexemeFilterInequality,
		lexemeFilterGreaterThan, lexemeFilterGreaterThanOrEqual,
		lexemeFilterLessThan, lexemeFilterLessThanOrEqual:
		return comparisonFilter(n)

	case lexemeFilterMatchesRegularExpression:
		return matchRegularExpression(n)

	case lexemeFilterNot:
		f := newFilter(n.children[0])
		return func(c *internal.Cursor) bool {
			return !f(c)
		}

	case lexemeFilterOr:
		f1 := newFilter(n.children[0])
		f2 := newFilter(n.children[1])
		return func(c *internal.Cursor) bool {
			return f1(c) || f2(c)
		}

	case lexemeFilterAnd:
		f1 := newFilter(n.children[0])
		f2 := newFilter(n.children[1])
		return func(c *internal.Cursor) bool {
			return f1(c) && f2(c)
		}

	case lexemeFilterBooleanLiteral:
		b, err := strconv.ParseBool(n.lexeme.val)
		if err != nil {
			panic(err) // should not happen
		}
		return func(c *internal.Cursor) bool {
			return b
		}

	default:
		return never
	}
}

// never is a filter function that always returns false, regardless of the input cursor.
func never(_ *internal.Cursor) bool {
	return false
}

// comparisonFilter creates a filter function that evaluates comparison operations in a filterNode.
// It ensures compatibility between operand types and performs type-specific comparisons.
// The resulting filter function evaluates node values in the context of the given filterNode's operator.
func comparisonFilter(n *filterNode) filter {
	compare := func(b bool) bool {
		var c comparison
		if b {
			c = compareEqual
		} else {
			c = compareIncomparable
		}
		return n.lexeme.comparator()(c)
	}
	return nodeToFilter(n, func(l, r typedValue) bool {
		if !l.typ.compatibleWith(r.typ) {
			return compare(false)
		}
		switch l.typ {
		case booleanValueType:
			return compare(equalBooleans(l.val, r.val))

		case nullValueType:
			return compare(equalNulls(l.val, r.val))

		default:
			return n.lexeme.comparator()(compareNodeValues(l, r))
		}
	})
}

// nodeToFilter converts a filterNode into a filter function that evaluates path-based comparisons using accept logic.
// It retrieves paths from the node's children, evaluates each path against the other using the accept function, and returns a match.
func nodeToFilter(n *filterNode, accept func(typedValue, typedValue) bool) filter {
	lhsPath := newFilterScanner(n.children[0])
	rhsPath := newFilterScanner(n.children[1])
	return func(c *internal.Cursor) (result bool) {
		// perform a set-wise comparison of the values in each path
		match := false
		for _, l := range lhsPath(c) {
			for _, r := range rhsPath(c) {
				if !accept(l, r) {
					return false
				}
				match = true
			}
		}
		return match
	}
}

// equalBooleans compares two string values for boolean equivalence, ignoring case sensitivity.
func equalBooleans(l, r string) bool {
	// Note: the YAML parser and our JSONPath lexer both rule out invalid boolean literals such as tRue.
	return strings.EqualFold(l, r)
}

// equalNulls compares two string representations of null values for equality without case sensitivity.
func equalNulls(l, r string) bool {
	// Note: the YAML parser and our JSONPath lexer both rule out invalid null literals such as nUll.
	return true
}

// filterScanner is a function that returns a slice of typed values from either a filter literal or a path expression
// which refers to either the current node or the root node. It is used in filter comparisons.
type filterScanner func(c *internal.Cursor) []typedValue

// emptyScanner is a function that returns an empty slice of typedValue, typically used as a default or fallback scanner.
func emptyScanner(_ *internal.Cursor) []typedValue {
	return []typedValue{}
}

// newFilterScanner returns a filterScanner based on the provided filterNode, delegating to specialized scanners or defaulting.
func newFilterScanner(n *filterNode) filterScanner {
	switch {
	case n == nil:
		return emptyScanner

	case n.isItemFilter():
		return pathFilterScanner(n)

	case n.isLiteral():
		return literalFilterScanner(n)

	default:
		return emptyScanner
	}
}

// pathFilterScanner creates a filterScanner for a filterNode representing either '@' or '$' path expressions.
// The scanner operates on the current cursor or root node, returning matched nodes from a generated path.
// Panics if the provided filterNode does not have a valid precondition for path scanning.
func pathFilterScanner(n *filterNode) filterScanner {
	var at bool
	switch n.lexeme.typ {
	case lexemeFilterAt:
		at = true
	case lexemeRoot:
		at = false
	default:
		panic("false precondition")
	}
	subpath := ""
	for _, lexeme := range n.subpath {
		subpath += lexeme.val
	}
	path, err := NewPath(subpath)
	if err != nil {
		return emptyScanner
	}
	return func(c *internal.Cursor) []typedValue {
		if at {
			nodes, err := path.Find(c.Node())
			return values(c, nodes, err)
		}
		nodes, err := path.Find(c.Root().Node())
		return values(c, nodes, err)
	}
}

type valueType int

const (
	unknownValueType valueType = iota
	stringValueType
	intValueType
	floatValueType
	booleanValueType
	nullValueType
	regularExpressionValueType
)

func (vt valueType) isNumeric() bool {
	return vt == intValueType || vt == floatValueType
}

func (vt valueType) compatibleWith(vt2 valueType) bool {
	return vt.isNumeric() && vt2.isNumeric() || vt == vt2 || vt == stringValueType && vt2 == regularExpressionValueType
}

type typedValue struct {
	typ valueType
	val string
}

const (
	nullTag  = "!!null"
	boolTag  = "!!bool"
	strTag   = "!!str"
	intTag   = "!!int"
	floatTag = "!!float"
)

func typedValueOfNode(node *yaml.Node) typedValue {
	var t valueType = unknownValueType
	if node.Kind == yaml.ScalarNode {
		switch node.ShortTag() {
		case nullTag:
			t = nullValueType

		case boolTag:
			t = booleanValueType

		case strTag:
			t = stringValueType

		case intTag:
			t = intValueType

		case floatTag:
			t = floatValueType
		}
	}

	return typedValue{
		typ: t,
		val: node.Value,
	}
}

// resolveAliasNode resolves alias nodes using the node's Alias pointer if present,
// otherwise falls back to looking up anchors on the cursor's root alias map.
// It follows alias chains up to a cap to avoid infinite loops.
func resolveAliasNode(c *internal.Cursor, n *yaml.Node) *yaml.Node {
	cur := n
	const maxDepth = 16
	for i := 0; cur != nil && cur.Kind == yaml.AliasNode && i < maxDepth; i++ {
		if cur.Alias != nil {
			cur = cur.Alias
			continue
		}
		aliases := c.Aliases()
		if aliases != nil {
			if anchored, ok := aliases[cur.Value]; ok && anchored != nil {
				cur = anchored
				continue
			}
		}
		// cannot resolve further
		break
	}
	return cur
}

// values converts a list of YAML nodes into a slice of typedValue, resolving alias nodes and skipping nil nodes.
// Panics if the provided error is non-nil, as this scenario should not occur.
func values(c *internal.Cursor, nodes []*yaml.Node, err error) []typedValue {
	if err != nil {
		panic(fmt.Errorf("unexpected error: %v", err)) // should never happen
	}
	v := []typedValue{}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		resolved := resolveAliasNode(c, n)
		if resolved == nil {
			continue
		}
		v = append(v, typedValueOfNode(resolved))
	}
	return v
}

// literalFilterScanner creates a filterScanner that evaluates a literal value from the given filterNode's lexeme.
func literalFilterScanner(n *filterNode) filterScanner {
	v := n.lexeme.literalValue()
	return func(_ *internal.Cursor) []typedValue {
		return []typedValue{v}
	}
}

// matchRegularExpression converts a parse tree node into a filter that performs regex-based string comparisons.
func matchRegularExpression(parseTree *filterNode) filter {
	return nodeToFilter(parseTree, stringMatchesRegularExpression)
}

// stringMatchesRegularExpression checks if a string value matches a given regular expression and returns true if matched.
// Returns false if the types of the inputs are not string and regular expression.
func stringMatchesRegularExpression(s, expr typedValue) bool {
	if s.typ != stringValueType || expr.typ != regularExpressionValueType {
		return false // can't compare types so return false
	}
	re, _ := regexp.Compile(expr.val) // regex already compiled during lexing
	return re.Match([]byte(s.val))
}
