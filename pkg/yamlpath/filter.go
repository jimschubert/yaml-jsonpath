package yamlpath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath/internal"
	"go.yaml.in/yaml/v3"
)

type filter func(c *internal.Cursor) bool

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

func never(_ *internal.Cursor) bool {
	return false
}

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

func equalBooleans(l, r string) bool {
	// Note: the YAML parser and our JSONPath lexer both rule out invalid boolean literals such as tRue.
	return strings.EqualFold(l, r)
}

func equalNulls(l, r string) bool {
	// Note: the YAML parser and our JSONPath lexer both rule out invalid null literals such as nUll.
	return true
}

// filterScanner is a function that returns a slice of typed values from either a filter literal or a path expression
// which refers to either the current node or the root node. It is used in filter comparisons.
type filterScanner func(c *internal.Cursor) []typedValue

func emptyScanner(_ *internal.Cursor) []typedValue {
	return []typedValue{}
}

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
	} else if node.Kind == yaml.AliasNode && node.Alias != nil {
		return typedValueOfNode(node.Alias)
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

// values now accepts the cursor so alias resolution can consult the root's alias map.
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

func literalFilterScanner(n *filterNode) filterScanner {
	v := n.lexeme.literalValue()
	return func(_ *internal.Cursor) []typedValue {
		return []typedValue{v}
	}
}

func matchRegularExpression(parseTree *filterNode) filter {
	return nodeToFilter(parseTree, stringMatchesRegularExpression)
}

func stringMatchesRegularExpression(s, expr typedValue) bool {
	if s.typ != stringValueType || expr.typ != regularExpressionValueType {
		return false // can't compare types so return false
	}
	re, _ := regexp.Compile(expr.val) // regex already compiled during lexing
	return re.Match([]byte(s.val))
}
