package gitops

import (
	"fmt"
	"math"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"
)

// headKeys lead every document, in this order; docs/spec/lua-render.md "YAML".
var headKeys = []string{"apiVersion", "kind", "metadata"}

// converter turns Lua values into YAML nodes; mapping keys come out sorted.
type converter struct {
	listMark *lua.LTable
}

// EncodeDocuments writes docs, a Lua list of document tables, as one multi-document YAML
// file, by the conversion rules in docs/spec/lua-render.md "YAML". listMark is the
// metatable infra-lua sets on a table it marks as a list, so an empty one is written as
// `[]`; every other empty table is written as `{}`. An error names the path to the value
// that has no YAML form, such as `[2].spec.template`.
func EncodeDocuments(docs lua.LValue, listMark *lua.LTable) ([]byte, error) {
	conv := converter{listMark: listMark}
	list, err := conv.node(docs, "")
	if err != nil {
		return nil, err
	}
	if list.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("documents: want a list of documents, got a %s", describeNode(list))
	}

	files := make([]string, 0, len(list.Content))
	for index, doc := range list.Content {
		if doc.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("[%d]: want a document map, got a %s", index+1, describeNode(doc))
		}
		out, err := yaml.Marshal(withHeadFirst(doc))
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", index+1, err)
		}
		files = append(files, string(out))
	}
	return []byte(strings.Join(files, "---\n")), nil
}

// withHeadFirst returns doc, a sorted mapping node, with headKeys moved to the front.
func withHeadFirst(doc *yaml.Node) *yaml.Node {
	head := make([]*yaml.Node, 0, len(headKeys)*2)
	for _, key := range headKeys {
		for i := 0; i < len(doc.Content); i += 2 {
			if doc.Content[i].Value == key {
				head = append(head, doc.Content[i], doc.Content[i+1])
			}
		}
	}

	rest := make([]*yaml.Node, 0, len(doc.Content)-len(head))
	for i := 0; i < len(doc.Content); i += 2 {
		if !isHeadKey(doc.Content[i].Value) {
			rest = append(rest, doc.Content[i], doc.Content[i+1])
		}
	}

	ordered := *doc
	ordered.Content = append(head, rest...)
	return &ordered
}

func isHeadKey(key string) bool {
	for _, head := range headKeys {
		if key == head {
			return true
		}
	}
	return false
}

func (c converter) node(value lua.LValue, at string) (*yaml.Node, error) {
	switch value := value.(type) {
	case lua.LString:
		return scalarNode(string(value))
	case lua.LBool:
		return scalarNode(bool(value))
	case lua.LNumber:
		return scalarNode(number(value))
	case *lua.LTable:
		return c.table(value, at)
	default:
		return nil, fmt.Errorf("%s: cannot write a %s to YAML", pathOrRoot(at), value.Type())
	}
}

// number is n as an int64 when its value is integral and fits, else as a float64.
func number(n lua.LNumber) any {
	f := float64(n)
	if f == math.Trunc(f) && f >= math.MinInt64 && f < math.MaxInt64 {
		return int64(f)
	}
	return f
}

func (c converter) table(table *lua.LTable, at string) (*yaml.Node, error) {
	var names []string
	var positions []float64
	var badKey lua.LValue
	table.ForEach(func(key, _ lua.LValue) {
		switch key := key.(type) {
		case lua.LString:
			names = append(names, string(key))
		case lua.LNumber:
			positions = append(positions, float64(key))
		default:
			badKey = key
		}
	})
	sort.Strings(names)

	switch {
	case badKey != nil:
		return nil, fmt.Errorf("%s: cannot write a table with a %s key to YAML", pathOrRoot(at), badKey.Type())
	case len(names) > 0 && len(positions) > 0:
		return nil, fmt.Errorf("%s: a table mixes list positions and map keys", pathOrRoot(at))
	case len(positions) > 0:
		return c.sequence(table, positions, at)
	case len(names) == 0 && c.listMark != nil && table.Metatable == lua.LValue(c.listMark):
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}, nil
	default:
		return c.mapping(table, names, at)
	}
}

// sequence converts table, whose number keys are positions, after checking they are
// exactly 1..n.
func (c converter) sequence(table *lua.LTable, positions []float64, at string) (*yaml.Node, error) {
	count := float64(len(positions))
	for _, position := range positions {
		if position != math.Trunc(position) || position < 1 || position > count {
			return nil, fmt.Errorf("%s: a list has keys other than 1..%d", pathOrRoot(at), len(positions))
		}
	}

	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for position := 1; position <= len(positions); position++ {
		item, err := c.node(table.RawGetInt(position), fmt.Sprintf("%s[%d]", at, position))
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, item)
	}
	return seq, nil
}

func (c converter) mapping(table *lua.LTable, names []string, at string) (*yaml.Node, error) {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, name := range names {
		key, err := scalarNode(name)
		if err != nil {
			return nil, err
		}
		item, err := c.node(table.RawGetString(name), at+"."+name)
		if err != nil {
			return nil, err
		}
		mapping.Content = append(mapping.Content, key, item)
	}
	return mapping, nil
}

// scalarNode lets yaml.v3 pick the tag and quoting for one Go scalar.
func scalarNode(value any) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return nil, err
	}
	return &node, nil
}

func describeNode(node *yaml.Node) string {
	switch node.Kind {
	case yaml.SequenceNode:
		return "list"
	case yaml.MappingNode:
		return "map"
	default:
		return strings.TrimPrefix(node.Tag, "!!")
	}
}

func pathOrRoot(at string) string {
	if at == "" {
		return "documents"
	}
	return at
}
