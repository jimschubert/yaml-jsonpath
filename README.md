# YAML JSONPath

[![GoDoc](https://godoc.org/github.com/vmware-labs/yaml-jsonpath?status.svg)](https://godoc.org/github.com/vmware-labs/yaml-jsonpath)
[![Go Report Card](https://goreportcard.com/badge/vmware-labs/yaml-jsonpath)](https://goreportcard.com/report/vmware-labs/yaml-jsonpath)

JSONPath implementation for the [YAML node](https://godoc.org/go.yaml.in/yaml/v3#Node) API.

## Syntax

Valid paths are strings conforming to the following BNF syntax.

```bnf
<path> ::= <identity> | <root> <subpath> | <subpath> |
           <undotted child> <subpath> | <subpath> <filter>         ; an undotted child is allowed at the start of a path
<identity> ::= ""                                                  ; the current node
<root> ::= "$"                                                     ; the root node of a document
<subpath> ::= <identity> | <child> <subpath> |
              <array access> <subpath> |
              <recursive descent> <subpath>

<child> ::= <dot child> | <bracket child>
<dot child> ::= "." <dotted child name> | ".*"                     ; named child (restricted characters) or all children
<bracket child> ::= "[" <child names> "]" | "[" <child names> "]~" ; named children | property names of children
<child names> ::= <child name> |
                  <child name> "," <child names> 
<undotted child> ::= <dotted child name> |                         ; named child (restricted characters)
                     <dotted child name><array access> |           ; array access of named child
                     <dotted child name>"~"                        ; property name of child
                    "*"                                            ; all children
                    "*" <array access>                             ; array access of all children
<child name> ::= "'" <single quoted string> "'" |
                 '"' <double quoted string> '"'
<single quoted string> ::= "\'" <single quoted string> |           ; escaped single quote
                           "\\" <single quoted string> |           ; escaped backslash
                           <string without ' or \> <single quoted string> |
                           ""                                      ; empty string
<double quoted string> ::= '\"' <double quoted string> |           ; escaped double quote
                           '\\' <double quoted string> |           ; escaped backslash
                           <string without " or \> <double quoted string> |
                           ""                                      ; empty string

<recursive descent> ::= ".." <dotted child name> |                 ; all the descendants named <dotted child name>
                        ".." <bracket child> |                     ; object access of all descendents
                        ".." <array access>  |                     ; array access of all descendents
<array access> ::= "[" "*" "]" | "[" union "]" | "[" <filter> "]"  ; all, zero or more elements of a sequence

<union> ::= <index> | <index> "," <union>
<index> ::= <integer> | <range>                                    ; specific index, range of indices, or all indices
<range> ::= <integer> ":" <integer> |                              ; start (inclusive) to end (exclusive)
            <integer> ":" <integer> ":" <integer>                  ; start (inclusive) to end (exclusive) by step

<filter> ::= "?(" <filter expr> ")"
<filter expr> ::= <filter and> |
                  <filter and> "||" <filter expr>                  ; disjunction
<filter and> ::= <basic filter> |
                <basic filter> "&&" <filter and>                   ; conjunction (binds more tightly than ||)
<basic filter> ::= <filter subpath> |                              ; subpath exists
                   "!" <basic filter> |                            ; negation
                   <filter term> "==" <filter term> |              ; equality
                   <filter term> "!=" <filter term> |              ; inequality
                   <filter term> ">" <filter term> |               ; numeric greater than
                   <filter term> ">=" <filter term> |              ; numeric greater than or equal to
                   <filter term> "<" <filter term> |               ; numeric less than
                   <filter term> "<=" <filter term> |              ; numeric less than or equal to
                   <filter subpath> "=~" <regular expr> |          ; subpath value matches regular expression
                   "(" <filter expr> ")"                           ; bracketing
<filter term> ::= "@" <subpath> |                                  ; item relative to element being processed
                  "@" |                                            ; value of element being processed
                  "$" <subpath> |                                  ; item relative to root node of a document
                  <filter literal>
<filter subpath> ::= "@" <subpath> |                               ; item, relative to element being processed
                     "$" <subpath>                                 ; item, relative to root node of a document
<filter literal> ::= <integer> |                                   ; positive or negative decimal integer
                     <floating point number> |                     ; floating point number
                     "'" <string without '> "'" |                  ; string enclosed in single quotes
                     "true" | "false" |                            ; boolean (must not be quoted)
                     "null"                                        ; null (must not be quoted)
<regular expr> ::= "/" <go regex> "/"                              ; Go regular expression with any "/" in the regex escaped as "\/"
```

The `NewPath` function parses a string path and returns a corresponding value of the `Path` type and
an error indicating whether parsing succeeded or failed.

Go regular expressions are defined [here](https://golang.org/pkg/regexp/).

## Semantics

The `Path` type's `Find` method takes a YAML node and returns a slice of descendants of the input node which match the Path. Each matching node appears at least once in the slice (but _may_ appear more than once).
If there are no matches, an empty slice is returned.

A path is logically a series of matchers. To start with, the first matcher is applied to a slice consisting of just the node which was input to the `Find` method. Each matcher is applied in turn to the slice of nodes found so far and the results are combined into a single slice, which then passes to the next matcher, and so on. If a matcher produces an
empty slice, then each subsequent matcher also produces an empty slice and the `Find` method returns an empty slice.

The following matchers, with corresponding concrete syntax, are supported. See the BNF syntax above for details of
the concrete syntax.

### Identity: empty string

This matches all the nodes in the input slice which therefore become the nodes of the matcher's output slice.
The identity matcher defines the behaviour of a path consisting of the empty string and is the only way
of terminating the `<subpath>` production in the BNF syntax.

### Root: `$`

This matches the root node of the input YAML node. This matcher may be specified only at the start of the path. It is optional and, if omitted, the root node is matched before the rest of the path is applied. The output slice consists of just the root node.

### Child: `.childname` or `['child', 'names', ...]`

This matches the children with the given names of all the mapping nodes in the input slice. The output slice consists of all those children. The given name may be a single child name (no periods) or a series of single child names separated by periods. Non-mapping nodes in the input slice are not matched.

Although either form `.childname` or `['childname']` accepts a child name with embedded spaces, the
`['childname']` form may be more convenient in some situations.

As a special case, `.*` also matches all the nodes in each sequence node in the input slice.

## Property Name

The Property Name Operator `~` can be included after a child name in the form of `.childname~`, `['childname']~` or `['childname1', "childname2"]~` to return the property name of the node instead of the value. this can only be used on the last part of the path

### Recursive Descent: `..childname` or `..*`

A matcher of the form `..childname` selects all the descendants of the nodes in the input slice (including those nodes) with the given name (using the same rules as the child matcher). The output slice consists of all the matching descendants.

A matcher of the form `..*` selects all the descendants of the nodes in the input slice (including those nodes).

### Array Subscript: `[integer]`, `[start:end]`, `[start:end:step]`, or `[*]`

This matches subsequences of all the sequence nodes in the input slice. Non-sequence nodes in the
input slice are not matched.

A matcher of the form `[integer]` selects the corresponding node in each sequence node, with `0` meaning the first node in the sequence, `1` the second node, and so on. A special index of `-1` selects the last node in each sequence.

A matcher of the form `[start:end]` or `[start:end:step]` selects the corresponding nodes in each sequence node starting from the start of the range (inclusive) to the end of the range (exclusive) with an optional step value (which defaults to `1`). A step value of `-1` may be used to step backwards from the end of the sequence to the
start.

A matcher of the form `[*]` selects all the nodes in each sequence node.

### Filters: `[?()]`

This matcher selects a subset of each node in the input satisfying the filter expression.

Filter expressions are composed of three kinds of term:

* `@` terms which produce a slice of descendants of the current node being matched (which is a node in one of the input sequences). Any path expression may be appended after the `@` to determine which descendants to include.
* `$` terms which produce a slice of descendants of the root node. Any path expression may be appended after the `$` to determine which descendants to include.
* Integer, floating point, and string literals (enclosed in single quotes, e.g. 'x').

Filter expressions combine terms into basic filters of various sorts:

* existence filters, which consist of just a `@` or `$` term, are true if and only if the given term produces a non-empty slice of descendants.
* comparison filters (`==`, `!=`, `>`, `>=`, `<`, `<=`, `=~`) are true if and only if the same comparison is true of the values of each pair of items produced by the terms on each side of the comparison except that an empty slice always compares as false.

Comparison filters are normally used to compare a term which produces a slice consisting of a single node and a literal. The value of the slice is compared to the literal and the result is the result of the comparison filter. For example, if `@.child` produces a slice with one node whose value is 3, then the filter `@.child<5` is true.

The more general case is a logical extension of this. Each value on the left hand side must pass the comparison with each value on the right hand side, except that if either side is empty, then the comparison filter
is false (because there were no matches on that side).

Comparison expressions are built from existence and/or comparison filters using familiar logical operators -- disjunction ("or", `||`), conjunction ("and", `&&`), and negation ("not", `!`) -- together with parenthesised expressions.

## Trying it out

See the [web application](cmd/web/README.md) provided in this repository.

## References

The following sources inspired the syntax and semantics of YAML JSONPath:

* [JSONPath - XPath for JSON](https://goessner.net/articles/JsonPath/) by Stefan Goessner
* [JSONPath Comparison](https://cburgmer.github.io/json-path-comparison/) by Christoph Burgmer
* [JSONPath Support](https://kubernetes.io/docs/reference/kubectl/jsonpath/) in the Kubernetes Reference documentation
* [JSONPath User Guide](https://unofficial-kubernetes.readthedocs.io/en/latest/user-guide/jsonpath/) in the Unofficial Kubernetes documentation
* [JSONPath Syntax](https://support.smartbear.com/alertsite/docs/monitors/api/endpoint/jsonpath.html) in the SmartBear AlertSite documentation
* [Parsing JSON is a Minefield 💣](http://seriot.ch/parsing_json.php) by Nicolas Seriot

## Developing

Run the tests as usual:

```sh
go test ./...
```

Fuzzing is also [available](./pkg/yamlpath/fuzz/README.md).

## Contributing

The yaml-jsonpath project team welcomes contributions from the community. If you wish to contribute code and you have not signed our contributor license agreement (CLA), our bot will update the issue when you open a Pull Request. For any questions about the CLA process, please refer to our [FAQ](https://cla.vmware.com/faq).

For more detailed information, refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License v2.0: see [LICENSE](./LICENSE) for details.
