/*
 * Copyright 2020 VMware, Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package yamlpath_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"go.yaml.in/yaml/v3"
)

func TestFind(t *testing.T) {
	y := `---
store:
  book:
    - category: reference
      author: Nigel Rees
      title: Sayings of the Century
      price: 8.95
    - category: fiction
      author: Evelyn Waugh
      title: Sword of Honour
      price: 12.99
    - category: fiction
      author: Herman Melville
      title: Moby Dick
      isbn: 0-553-21311-3
      price: 8.99
    - category: fiction
      author: J. R. R. Tolkien
      title: The Lord of the Rings
      isbn: 0-395-19395-8
      price: 22.99
  bicycle:
    color: red
    price: 19.95
  feather duster:
    price: 9.95
x:
  - y:
    - z: 1
      w: 2
  - y:
    - z: 3
      w: 4
test~: hello world
test: this is a test
`
	var n yaml.Node

	err := yaml.Unmarshal([]byte(y), &n)
	require.NoError(t, err)

	cases := []struct {
		name            string
		path            string
		expectedStrings []string
		expectedPathErr string
		focus           bool // if true, run only tests with focus set to true
	}{
		{
			name: "property names",
			path: "$.store~",
			expectedStrings: []string{
				`store
`,
			},
			expectedPathErr: "",
		},
		{
			name: "property names bracket child",
			path: "$.store['book']~",
			expectedStrings: []string{
				`book
`,
			},
			expectedPathErr: "",
		},
		{
			name: "property names bracket children",
			path: "$.store.book[0]['category','author']~",
			expectedStrings: []string{
				`category
`,
				`author
`,
			},
			expectedPathErr: "",
		},
		{
			name: "property names arraysubscript",
			path: "$.store.book[0][*]~",
			expectedStrings: []string{
				`category
`,
				`author
`,
				`title
`,
				`price
`,
			},
			expectedPathErr: "",
		},
		{
			name: "property names bracket child with ~ in name",
			path: "$['test~']~",
			expectedStrings: []string{
				`test~
`,
			},
			expectedPathErr: "",
		},
		{
			name: "dotted child with ~ in name",
			path: "$.test~",
			expectedStrings: []string{
				`test
`,
			},
			expectedPathErr: "",
		},
		{
			name: "identity",
			path: "",
			expectedStrings: []string{`store:
  book:
    - category: reference
      author: Nigel Rees
      title: Sayings of the Century
      price: 8.95
    - category: fiction
      author: Evelyn Waugh
      title: Sword of Honour
      price: 12.99
    - category: fiction
      author: Herman Melville
      title: Moby Dick
      isbn: 0-553-21311-3
      price: 8.99
    - category: fiction
      author: J. R. R. Tolkien
      title: The Lord of the Rings
      isbn: 0-395-19395-8
      price: 22.99
  bicycle:
    color: red
    price: 19.95
  feather duster:
    price: 9.95
x:
  - y:
      - z: 1
        w: 2
  - y:
      - z: 3
        w: 4
test~: hello world
test: this is a test
`},
			expectedPathErr: "",
		},
		{
			name: "root",
			path: "$",
			expectedStrings: []string{`store:
  book:
    - category: reference
      author: Nigel Rees
      title: Sayings of the Century
      price: 8.95
    - category: fiction
      author: Evelyn Waugh
      title: Sword of Honour
      price: 12.99
    - category: fiction
      author: Herman Melville
      title: Moby Dick
      isbn: 0-553-21311-3
      price: 8.99
    - category: fiction
      author: J. R. R. Tolkien
      title: The Lord of the Rings
      isbn: 0-395-19395-8
      price: 22.99
  bicycle:
    color: red
    price: 19.95
  feather duster:
    price: 9.95
x:
  - y:
      - z: 1
        w: 2
  - y:
      - z: 3
        w: 4
test~: hello world
test: this is a test
`},
			expectedPathErr: "",
		},
		{
			name: "dot child",
			path: "$.store",
			expectedStrings: []string{`book:
  - category: reference
    author: Nigel Rees
    title: Sayings of the Century
    price: 8.95
  - category: fiction
    author: Evelyn Waugh
    title: Sword of Honour
    price: 12.99
  - category: fiction
    author: Herman Melville
    title: Moby Dick
    isbn: 0-553-21311-3
    price: 8.99
  - category: fiction
    author: J. R. R. Tolkien
    title: The Lord of the Rings
    isbn: 0-395-19395-8
    price: 22.99
bicycle:
  color: red
  price: 19.95
feather duster:
  price: 9.95
`},
			expectedPathErr: "",
		},
		{
			name: "dot child with implicit root",
			path: ".store",
			expectedStrings: []string{`book:
  - category: reference
    author: Nigel Rees
    title: Sayings of the Century
    price: 8.95
  - category: fiction
    author: Evelyn Waugh
    title: Sword of Honour
    price: 12.99
  - category: fiction
    author: Herman Melville
    title: Moby Dick
    isbn: 0-553-21311-3
    price: 8.99
  - category: fiction
    author: J. R. R. Tolkien
    title: The Lord of the Rings
    isbn: 0-395-19395-8
    price: 22.99
bicycle:
  color: red
  price: 19.95
feather duster:
  price: 9.95
`},
			expectedPathErr: "",
		},
		{
			name: "undotted child with implicit root",
			path: "store",
			expectedStrings: []string{`book:
  - category: reference
    author: Nigel Rees
    title: Sayings of the Century
    price: 8.95
  - category: fiction
    author: Evelyn Waugh
    title: Sword of Honour
    price: 12.99
  - category: fiction
    author: Herman Melville
    title: Moby Dick
    isbn: 0-553-21311-3
    price: 8.99
  - category: fiction
    author: J. R. R. Tolkien
    title: The Lord of the Rings
    isbn: 0-395-19395-8
    price: 22.99
bicycle:
  color: red
  price: 19.95
feather duster:
  price: 9.95
`},
			expectedPathErr: "",
		},
		{
			name: "undotted all children with implicit root",
			path: "*",
			expectedStrings: []string{
				`book:
  - category: reference
    author: Nigel Rees
    title: Sayings of the Century
    price: 8.95
  - category: fiction
    author: Evelyn Waugh
    title: Sword of Honour
    price: 12.99
  - category: fiction
    author: Herman Melville
    title: Moby Dick
    isbn: 0-553-21311-3
    price: 8.99
  - category: fiction
    author: J. R. R. Tolkien
    title: The Lord of the Rings
    isbn: 0-395-19395-8
    price: 22.99
bicycle:
  color: red
  price: 19.95
feather duster:
  price: 9.95
`,
				`- y:
    - z: 1
      w: 2
- y:
    - z: 3
      w: 4
`,
				`hello world
`,
				`this is a test
`,
			},
			expectedPathErr: "",
		},
		{
			name:            "dot child with no name",
			path:            "$.",
			expectedPathErr: `child name missing at position 2, following "$."`,
		},
		{
			name:            "dot child with trailing dot",
			path:            "$.store.",
			expectedPathErr: `child name missing at position 8, following ".store."`,
		},
		{
			name: "dot child of dot child",
			path: "$.store.book",
			expectedStrings: []string{`- category: reference
  author: Nigel Rees
  title: Sayings of the Century
  price: 8.95
- category: fiction
  author: Evelyn Waugh
  title: Sword of Honour
  price: 12.99
- category: fiction
  author: Herman Melville
  title: Moby Dick
  isbn: 0-553-21311-3
  price: 8.99
- category: fiction
  author: J. R. R. Tolkien
  title: The Lord of the Rings
  isbn: 0-395-19395-8
  price: 22.99
`},
			expectedPathErr: "",
		},
		{
			name: "dot child with embedded wildcard",
			path: "$.store.*.color",
			expectedStrings: []string{
				"red\n",
			},
			expectedPathErr: "",
		},
		{
			name:            "dot child with embedded space",
			path:            "$.store.feather duster.price",
			expectedPathErr: `invalid character ' ' at position 15, following ".feather"`,
		},
		{
			name: "bracket child",
			path: "$['store']",
			expectedStrings: []string{`book:
  - category: reference
    author: Nigel Rees
    title: Sayings of the Century
    price: 8.95
  - category: fiction
    author: Evelyn Waugh
    title: Sword of Honour
    price: 12.99
  - category: fiction
    author: Herman Melville
    title: Moby Dick
    isbn: 0-553-21311-3
    price: 8.99
  - category: fiction
    author: J. R. R. Tolkien
    title: The Lord of the Rings
    isbn: 0-395-19395-8
    price: 22.99
bicycle:
  color: red
  price: 19.95
feather duster:
  price: 9.95
`},
			expectedPathErr: "",
		},
		{
			name: "bracket child with double quotes",
			path: `$["store"]`,
			expectedStrings: []string{`book:
  - category: reference
    author: Nigel Rees
    title: Sayings of the Century
    price: 8.95
  - category: fiction
    author: Evelyn Waugh
    title: Sword of Honour
    price: 12.99
  - category: fiction
    author: Herman Melville
    title: Moby Dick
    isbn: 0-553-21311-3
    price: 8.99
  - category: fiction
    author: J. R. R. Tolkien
    title: The Lord of the Rings
    isbn: 0-395-19395-8
    price: 22.99
bicycle:
  color: red
  price: 19.95
feather duster:
  price: 9.95
`},
			expectedPathErr: "",
		},
		{
			name: "bracket child of bracket child",
			path: "$['store']['book']",
			expectedStrings: []string{`- category: reference
  author: Nigel Rees
  title: Sayings of the Century
  price: 8.95
- category: fiction
  author: Evelyn Waugh
  title: Sword of Honour
  price: 12.99
- category: fiction
  author: Herman Melville
  title: Moby Dick
  isbn: 0-553-21311-3
  price: 8.99
- category: fiction
  author: J. R. R. Tolkien
  title: The Lord of the Rings
  isbn: 0-395-19395-8
  price: 22.99
`},
			expectedPathErr: "",
		},
		{
			name:            "bracket dotted child",
			path:            "$['store.book']",
			expectedStrings: []string{},
			expectedPathErr: "",
		},
		{
			name: "bracket child with embedded space",
			path: "$.store['feather duster'].price",
			expectedStrings: []string{
				"9.95\n",
			},
			expectedPathErr: "",
		},
		{
			name: "bracket child of dot child",
			path: "$.store['book']",
			expectedStrings: []string{`- category: reference
  author: Nigel Rees
  title: Sayings of the Century
  price: 8.95
- category: fiction
  author: Evelyn Waugh
  title: Sword of Honour
  price: 12.99
- category: fiction
  author: Herman Melville
  title: Moby Dick
  isbn: 0-553-21311-3
  price: 8.99
- category: fiction
  author: J. R. R. Tolkien
  title: The Lord of the Rings
  isbn: 0-395-19395-8
  price: 22.99
`},
			expectedPathErr: "",
		},
		{
			name: "dot child of bracket child",
			path: "$['store'].book",
			expectedStrings: []string{`- category: reference
  author: Nigel Rees
  title: Sayings of the Century
  price: 8.95
- category: fiction
  author: Evelyn Waugh
  title: Sword of Honour
  price: 12.99
- category: fiction
  author: Herman Melville
  title: Moby Dick
  isbn: 0-553-21311-3
  price: 8.99
- category: fiction
  author: J. R. R. Tolkien
  title: The Lord of the Rings
  isbn: 0-395-19395-8
  price: 22.99
`},
			expectedPathErr: "",
		},
		{
			name:            "unclosed bracket child",
			path:            "$['store",
			expectedPathErr: `unmatched "'" at position 8, following "$['store"`,
		},
		{
			name: "recursive descent",
			path: "$..price",
			expectedStrings: []string{
				"8.95\n",
				"12.99\n",
				"8.99\n",
				"22.99\n",
				"19.95\n",
				"9.95\n",
			},
			expectedPathErr: "",
		},
		{
			name: "recursive descent of dot child",
			path: "$.store.book..price",
			expectedStrings: []string{
				"8.95\n",
				"12.99\n",
				"8.99\n",
				"22.99\n",
			},
			expectedPathErr: "",
		},
		{
			name: "recursive descent of child starting with undotted implicit root",
			path: "store.book..price",
			expectedStrings: []string{
				"8.95\n",
				"12.99\n",
				"8.99\n",
				"22.99\n",
			},
			expectedPathErr: "",
		},
		{
			name: "recursive descent of bracket child",
			path: "$['store']['book']..price",
			expectedStrings: []string{
				"8.95\n",
				"12.99\n",
				"8.99\n",
				"22.99\n",
			},
			expectedPathErr: "",
		},
		{
			name: "recursive descent with wildcard",
			path: "$.store.bicycle..*",
			expectedStrings: []string{
				"red\n",
				"19.95\n",
			},
			expectedPathErr: "",
		},
		{
			name: "repeated recursive descent",
			path: "$..book..price",
			expectedStrings: []string{
				"8.95\n",
				"12.99\n",
				"8.99\n",
				"22.99\n",
			},
			expectedPathErr: "",
		},
		{
			name: "recursive descent with dot child",
			path: "$..bicycle.color",
			expectedStrings: []string{
				"red\n",
			},
			expectedPathErr: "",
		},
		{
			name: "recursive descent with bracket child",
			path: "$..bicycle['color']",
			expectedStrings: []string{
				"red\n",
			},
			expectedPathErr: "",
		},
		{
			name:            "recursive descent with missing name",
			path:            "$..",
			expectedPathErr: `child name or array access or filter missing after recursive descent at position 3, following "$.."`,
		},
		{
			name: "dot wildcarded children",
			path: "$.store.bicycle.*",
			expectedStrings: []string{
				"red\n",
				"19.95\n",
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript wildcard",
			path: "$.store.book[*]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
				`category: fiction
author: J. R. R. Tolkien
title: The Lord of the Rings
isbn: 0-395-19395-8
price: 22.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript single",
			path: "$.store.book[0]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript from:to",
			path: "$.store.book[1:3]",
			expectedStrings: []string{
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript from:to:step",
			path: "$.store.book[0:3:2]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript :to",
			path: "$.store.book[:2]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript ::step",
			path: "$.store.book[::2]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript from:to:",
			path: "$.store.book[1:3:]",
			expectedStrings: []string{
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript ::",
			path: "$.store.book[::]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
				`category: fiction
author: J. R. R. Tolkien
title: The Lord of the Rings
isbn: 0-395-19395-8
price: 22.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript ::-1",
			path: "$.store.book[::-1]",
			expectedStrings: []string{
				`category: fiction
author: J. R. R. Tolkien
title: The Lord of the Rings
isbn: 0-395-19395-8
price: 22.99
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript -3:-1",
			path: "$.store.book[-3:-1]",
			expectedStrings: []string{
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "array subscript -1:",
			path: "$.store.book[-1:]",
			expectedStrings: []string{
				`category: fiction
author: J. R. R. Tolkien
title: The Lord of the Rings
isbn: 0-395-19395-8
price: 22.99
`,
			},
			expectedPathErr: "",
		},
		{
			name:            "missing array subscript",
			path:            "$.store.book[]",
			expectedStrings: []string{},
			expectedPathErr: "subscript missing from [] before position 14",
		},
		{
			name:            "malformed array subscript",
			path:            "$.store.book[::0]",
			expectedStrings: []string{},
			expectedPathErr: "invalid array index [::0] before position 17: array index step value must be non-zero",
		},
		{
			name:            "array subscript out of bounds",
			path:            "$.store.book[99]",
			expectedStrings: []string{},
			expectedPathErr: "",
		},
		{
			name: "filter >",
			path: "$.store.book[?(@.price > 8.98)]",
			expectedStrings: []string{
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
				`category: fiction
author: Herman Melville
title: Moby Dick
isbn: 0-553-21311-3
price: 8.99
`,
				`category: fiction
author: J. R. R. Tolkien
title: The Lord of the Rings
isbn: 0-395-19395-8
price: 22.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "filter ==",
			path: "$.store.book[?(@.category == 'reference')]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
			},
			expectedPathErr: "",
		},
		{
			name: "filter == with bracket child",
			path: "$.store.book[?(@.category == 'reference')]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
			},
			expectedPathErr: "",
		},
		{
			name: "filter !=",
			path: "$.store.book[?(@.category != 'fiction')]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
			},
			expectedPathErr: "",
		},
		{
			name: "filter involving root",
			path: "$.store.book[?(@.price > $.store.bicycle.price)]",
			expectedStrings: []string{`category: fiction
author: J. R. R. Tolkien
title: The Lord of the Rings
isbn: 0-395-19395-8
price: 22.99
`},
			expectedPathErr: "",
		},
		{
			name: "nested filter (edge case)",
			path: "$.x[?(@.y[?(@.z==1)].w==2)]",
			expectedStrings: []string{
				`y:
  - z: 1
    w: 2
`,
			},
			expectedPathErr: "",
		},
		{
			name: "negated filter",
			path: "$.store.book[?(!@.isbn)]",
			expectedStrings: []string{
				`category: reference
author: Nigel Rees
title: Sayings of the Century
price: 8.95
`,
				`category: fiction
author: Evelyn Waugh
title: Sword of Honour
price: 12.99
`,
			},
			expectedPathErr: "",
		},
		{
			name: "map filter",
			path: `$.store.bicycle[?(@.color == "red")]`,
			expectedStrings: []string{
				`color: red
price: 19.95
`,
			},
		},
	}

	focussed := false
	for _, tc := range cases {
		if tc.focus {
			focussed = true
			break
		}
	}

	for _, tc := range cases {
		if focussed && !tc.focus {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			p, err := yamlpath.NewPath(tc.path)
			if err != nil {
				require.Nil(t, p)
			}
			if tc.expectedPathErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedPathErr)
				return
			}

			actual, err := p.Find(&n)
			require.NoError(t, err)

			actualStrings := []string{}
			for _, a := range actual {
				var buf bytes.Buffer
				e := yaml.NewEncoder(&buf)
				e.SetIndent(2)

				err = e.Encode(a)
				require.NoError(t, err)
				e.Close()
				actualStrings = append(actualStrings, buf.String())
			}

			require.Equal(t, tc.expectedStrings, actualStrings)
		})
	}

	if focussed {
		t.Fatalf("testcase(s) still focussed")
	}
}

func TestFindOtherDocuments(t *testing.T) {
	cases := []struct {
		name            string
		input           string
		path            string
		expectedStrings []string
		expectedPathErr string
		focus           bool // if true, run only tests with focus set to true
	}{
		{
			name:            "empty document",
			expectedStrings: []string{},
		},
		{
			name: "document with values matching keys",
			input: `c: a
a: b`,
			path:            ".a",
			expectedStrings: []string{"b\n"},
		},
		{
			name: "document with top-level array",
			input: `- c: a
- a: b`,
			path:            "$[0]",
			expectedStrings: []string{"c: a\n"},
		},
		{
			name: "document with top-level array, .*",
			input: `- c: a
- a: b`,
			path:            "$.*",
			expectedStrings: []string{"c: a\n", "a: b\n"},
		},
		{
			name: "document with top-level array, filter with double-quoted string literal",
			input: `- c: a
- a: b`,
			path:            `$[?(@.c=="a")]`,
			expectedStrings: []string{"c: a\n"},
		},
		{
			name: "union with keys",
			input: `key: value
another: entry`,
			path:            `$['key','another']`,
			expectedStrings: []string{"value\n", "entry\n"},
		},
		{
			name: "bracket child with quoted union literal",
			input: `",": value
another: entry`,
			path:            `$[',']`,
			expectedStrings: []string{"value\n"},
		},
		{
			name:            "array access after recursive descent",
			input:           `{"k": [{"key": "some value"}, {"key": 42}], "kk": [[{"key": 100}, {"key": 200}, {"key": 300}], [{"key": 400}, {"key": 500}, {"key": 600}]], "key": [0, 1]}`,
			path:            `$..[1].key`,
			expectedStrings: []string{"42\n", "200\n", "500\n"},
		},
		{
			name:            "filter after recursive descent",
			input:           `{"k": [{"key": "some value"}, {"key": 42}], "kk": [[{"key": 100}, {"key": 200}, {"key": 300}], [{"key": 400}, {"key": 500}, {"key": 600}]], "key": [0, 1]}`,
			path:            `$..[?(@.key>=500)]`,
			expectedStrings: []string{"{\"key\": 500}\n", "{\"key\": 600}\n"},
		},
		{
			name:            "union with wildcard and numbers (deviation from comparison project consensus)",
			input:           `["a","b","c"]`,
			path:            `$[*,1,0,*]`,
			expectedPathErr: `invalid array index [*,1,0,*] before position 10: error in union member 0: wildcard cannot be used in union`,
		},
		{
			name:            "special characters in bracket child name",
			input:           `{":@.\"$,*'\\": 42}`,
			path:            `$[':@."$,*\'\\']`,
			expectedStrings: []string{"42\n"},
		},
		{
			name:  "filter with boolean value comparison",
			input: `[{"a":true, "b": 1}, {"a":"true", "b": 2}]`,
			path:  `$[?(@.a==true)]`,
			expectedStrings: []string{`{"a": true, "b": 1}
`},
		},
		{
			name:  "filter with null value comparison",
			input: `[{"a":null, "b": 1}, {"a":"null", "b": 2}]`,
			path:  `$[?(@.a==null)]`,
			expectedStrings: []string{`{"a": null, "b": 1}
`},
		},
		{
			name:  "filter with integer that appears in string",
			input: `[{"a": "42", "b": 1}]`,
			path:  `$[?(@.a!=42)]`,
			expectedStrings: []string{`{"a": "42", "b": 1}
`},
		},
		{
			name:            "filter involving value of current node",
			input:           `[0,42,100]`,
			path:            `$[?(@>=42)]`,
			expectedStrings: []string{"42\n", "100\n"},
		},
		{
			name:            "filter with fractional float",
			input:           `[0,-4.2,100]`,
			path:            `$[?(@==-42E-1)]`,
			expectedStrings: []string{"-4.2\n"},
		},
		{
			name:            "filter with boolean predicate",
			input:           `[0]`,
			path:            `$[?(true)]`,
			expectedStrings: []string{"0\n"},
		},
		{
			name:            "relaxed spelling of true, false, and null literals", // See https://yaml.org/spec/1.2/spec.html#id2805071
			input:           `[FALSE, False, false, fAlse, TRUE, True, true, tRue, NULL, Null, null, nUll]`,
			path:            `$[?(@==false || @==true || @==null)]`,
			expectedStrings: []string{"FALSE\n", "False\n", "false\n", "TRUE\n", "True\n", "true\n", "NULL\n", "Null\n", "null\n"},
		},
	}

	focussed := false
	for _, tc := range cases {
		if tc.focus {
			focussed = true
			break
		}
	}

	for _, tc := range cases {
		if focussed && !tc.focus {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			var n yaml.Node
			err := yaml.Unmarshal([]byte(tc.input), &n)
			require.NoError(t, err)

			p, err := yamlpath.NewPath(tc.path)
			if tc.expectedPathErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedPathErr)
				return
			}

			actual, err := p.Find(&n)
			require.NoError(t, err)

			actualStrings := []string{}
			for _, a := range actual {
				var buf bytes.Buffer
				e := yaml.NewEncoder(&buf)
				e.SetIndent(2)

				err = e.Encode(a)
				require.NoError(t, err)
				e.Close()
				actualStrings = append(actualStrings, buf.String())
			}

			require.Equal(t, tc.expectedStrings, actualStrings)
		})
	}

	if focussed {
		t.Fatalf("testcase(s) still focussed")
	}
}

func TestFindAnchorsAndAliases(t *testing.T) {
	cases := []struct {
		name            string
		input           string
		path            string
		expectedStrings []string
	}{
		{
			name: "simple alias resolution",
			input: `
a: &anchor
  value: 42
b: *anchor
`,
			path:            "$.b.value",
			expectedStrings: []string{"42\n"},
		},
		{
			name: "alias inside sequence",
			input: `
defaults: &defaults
  color: red
items:
  - name: item1
    <<: *defaults
  - name: item2
    color: blue
`,
			path:            "$.items[0].color",
			expectedStrings: []string{"red\n"},
		},
		{
			name: "recursive alias reference",
			input: `
foo: &foo
  bar: &bar
    baz: 123
ref: *foo
`,
			path:            "$.ref.bar.baz",
			expectedStrings: []string{"123\n"},
		},
		{
			name: "three deep alias chain resolves value",
			input: `
a: &base
  key: value
b: &b
  <<: *base
c: &c
  <<: *b
d: *c
`,
			path:            `$.d.key`,
			expectedStrings: []string{"value\n"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var n yaml.Node
			err := yaml.Unmarshal([]byte(tc.input), &n)
			require.NoError(t, err)

			p, err := yamlpath.NewPath(tc.path)
			require.NoError(t, err)

			actual, err := p.Find(&n)
			require.NoError(t, err)

			actualStrings := []string{}
			for _, a := range actual {
				var buf bytes.Buffer
				e := yaml.NewEncoder(&buf)
				e.SetIndent(2)
				err = e.Encode(a)
				require.NoError(t, err)
				e.Close()
				actualStrings = append(actualStrings, buf.String())
			}

			require.Equal(t, tc.expectedStrings, actualStrings)
		})
	}
}

func TestNewPathWithRoot_AliasResolution(t *testing.T) {
	yamlData := `
root: &shared
  value: 42
a: *shared
b: *shared
c:
  nested: *shared
`
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(yamlData), &root))

	// Path to 'a.value'
	p1, err := yamlpath.NewPathWithRoot("$.a.value", &root)
	require.NoError(t, err)
	nodes1, err := p1.Find(&root)
	require.NoError(t, err)
	require.Len(t, nodes1, 1)
	require.Equal(t, "42", nodes1[0].Value)

	// Path to 'b.value'
	p2, err := yamlpath.NewPathWithRoot("$.b.value", &root)
	require.NoError(t, err)
	nodes2, err := p2.Find(&root)
	require.NoError(t, err)
	require.Len(t, nodes2, 1)
	require.Equal(t, "42", nodes2[0].Value)

	// Path to 'c.nested.value'
	p3, err := yamlpath.NewPathWithRoot("$.c.nested.value", &root)
	require.NoError(t, err)
	nodes3, err := p3.Find(&root)
	require.NoError(t, err)
	require.Len(t, nodes3, 1)
	require.Equal(t, "42", nodes3[0].Value)
}

func TestFindOnChildNodesWithRoot(t *testing.T) {
	yamlData := `
root: &shared
  value: 42
a: *shared
b: *shared
c:
  nested: *shared
`
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(yamlData), &root))

	// Helper to find a mapping child by key
	findChild := func(parent *yaml.Node, key string) *yaml.Node {
		for i := 0; i < len(parent.Content); i += 2 {
			if parent.Content[i].Value == key {
				return parent.Content[i+1]
			}
		}
		return nil
	}

	aNode := findChild(root.Content[0], "a")
	bNode := findChild(root.Content[0], "b")
	cNode := findChild(root.Content[0], "c")

	p, err := yamlpath.NewPathWithRoot("$.value", &root)
	require.NoError(t, err)

	nodesA, err := p.Find(aNode)
	require.NoError(t, err)
	require.Len(t, nodesA, 1)
	require.Equal(t, "42", nodesA[0].Value)

	nodesB, err := p.Find(bNode)
	require.NoError(t, err)
	require.Len(t, nodesB, 1)
	require.Equal(t, "42", nodesB[0].Value)

	// For c.nested
	nestedNode := findChild(cNode, "nested")
	nodesC, err := p.Find(nestedNode)
	require.NoError(t, err)
	require.Len(t, nodesC, 1)
	require.Equal(t, "42", nodesC[0].Value)
}

func TestFindStoreBookGenres(t *testing.T) {
	yamlData := `
defaults:
  genres:
    fiction: &fiction
      genre: Fiction
    science-fiction: &science-fiction
      genre: Science Fiction
    fantasy: &fantasy
      genre: Fantasy
  hemingway: &hemingway
    author: Ernest Hemingway
store:
  book: &books
    - <<: [*hemingway, *fiction]
      title: The Old Man and the Sea
    - <<: [*hemingway, *fiction]
      title: For Whom the Bell Tolls
    - <<: [*hemingway, *fiction]
      title: To Have and Have Not
      <<: *fiction
    - author: Fyodor Mikhailovich Dostoevsky
      title: Crime and Punishment
      <<: *fiction
    - author: Jane Austen
      title: Sense and Sensibility
      <<: *fiction
    - author: Kurt Vonnegut Jr.
      title: Slaughterhouse-Five
      <<: *science-fiction
    - author: J. R. R. Tolkien
      title: The Lord of the Rings
      <<: *fantasy
  audiobooks:
    - *books
    - author: Stephen "Steve-O" Glover
      title: 'Professional Idiot: A Memoir'
`

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(yamlData), &root))

	p, err := yamlpath.NewPathWithRoot("$.store.book[*].genre", &root)
	require.NoError(t, err)

	actualNodes, err := p.Find(&root)
	require.NoError(t, err)

	actualStrings := []string{}
	for _, a := range actualNodes {
		var buf bytes.Buffer
		e := yaml.NewEncoder(&buf)
		e.SetIndent(2)
		require.NoError(t, e.Encode(a))
		e.Close()
		actualStrings = append(actualStrings, buf.String())
	}

	expected := []string{
		"Fiction\n",
		"Fiction\n",
		"Fiction\n",
		"Fiction\n",
		"Fiction\n",
		"Science Fiction\n",
		"Fantasy\n",
	}

	require.Equal(t, expected, actualStrings)
}
