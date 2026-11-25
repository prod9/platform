package gitops

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"
)

// decoder turns YAML nodes into Lua values in one state, the inverse of converter.
type decoder struct {
	state    *lua.LState
	listMark *lua.LTable
}

// decodeDocuments reads a multi-document YAML stream into a Lua list of document tables,
// by the rules in docs/spec/lua-render.md "Upstream manifests": empty documents are
// skipped, null mapping values dropped, and an empty sequence carries listMark when there
// is one. An error names the document and the path to the value that has no Lua form.
func decodeDocuments(state *lua.LState, stream []byte, listMark *lua.LTable) (*lua.LTable, error) {
	dec := decoder{state: state, listMark: listMark}
	yamlDec := yaml.NewDecoder(bytes.NewReader(stream))

	docs := state.NewTable()
	for index := 1; ; index++ {
		var doc yaml.Node
		err := yamlDec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(doc.Content) == 0 || isNull(doc.Content[0]) {
			continue
		}

		value, err := dec.value(doc.Content[0], fmt.Sprintf("document %d", index))
		if err != nil {
			return nil, err
		}
		docs.Append(value)
	}
	return docs, nil
}

func (d decoder) value(node *yaml.Node, at string) (lua.LValue, error) {
	switch node.Kind {
	case yaml.AliasNode:
		return d.value(node.Alias, at)
	case yaml.MappingNode:
		return d.mapping(node, at)
	case yaml.SequenceNode:
		return d.sequence(node, at)
	default:
		return scalar(node, at)
	}
}

func (d decoder) mapping(node *yaml.Node, at string) (lua.LValue, error) {
	table := d.state.NewTable()
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, item := node.Content[i].Value, node.Content[i+1]
		if isNull(item) {
			continue
		}
		value, err := d.value(item, at+"."+key)
		if err != nil {
			return nil, err
		}
		table.RawSetString(key, value)
	}
	return table, nil
}

func (d decoder) sequence(node *yaml.Node, at string) (lua.LValue, error) {
	table := d.state.NewTable()
	for index, item := range node.Content {
		itemAt := fmt.Sprintf("%s[%d]", at, index+1)
		if isNull(item) {
			return nil, fmt.Errorf("%s: a null list item has no Lua form", itemAt)
		}
		value, err := d.value(item, itemAt)
		if err != nil {
			return nil, err
		}
		table.Append(value)
	}
	if len(node.Content) == 0 && d.listMark != nil {
		table.Metatable = d.listMark
	}
	return table, nil
}

// scalar keeps a YAML scalar's type: booleans and numbers as themselves, and every other
// tag, timestamps included, as its string.
func scalar(node *yaml.Node, at string) (lua.LValue, error) {
	switch node.ShortTag() {
	case "!!bool":
		var b bool
		if err := node.Decode(&b); err != nil {
			return nil, fmt.Errorf("%s: %w", at, err)
		}
		return lua.LBool(b), nil
	case "!!int", "!!float":
		var n float64
		if err := node.Decode(&n); err != nil {
			return nil, fmt.Errorf("%s: %w", at, err)
		}
		return lua.LNumber(n), nil
	default:
		return lua.LString(node.Value), nil
	}
}

func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.ShortTag() == "!!null"
}
