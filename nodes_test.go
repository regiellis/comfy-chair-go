package main

import "testing"

func TestReplacePlaceholders(t *testing.T) {
	values := map[string]string{
		"{{NodeName}}":      "MyNode",
		"{{NodeNameLower}}": "mynode",
		"{{NodeDesc}}":      "", // empty values are stripped, not substituted
	}

	// Mixed quoted and bare occurrences in the same content must all be
	// replaced (a previous implementation replaced only the quoted ones).
	in := `class {{NodeName}}(IO.ComfyNode):
    node_id="{{NodeName}}"
    desc="{{NodeDesc}}"
    from .src.{{NodeNameLower}} import {{NodeName}}`
	want := `class MyNode(IO.ComfyNode):
    node_id="MyNode"
    desc=""
    from .src.mynode import MyNode`

	if got := replacePlaceholders(in, values); got != want {
		t.Errorf("replacePlaceholders mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
