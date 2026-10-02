package decoders

import (
	"github.com/zclconf/go-cty/cty"
)

const DefaultGlazeElementName = "default"

// Base holds the attributes that sessions, windows and panes share.
type Base struct {
	Name              string
	Hooks             map[string]string
	Options           map[string]string
	StartingDirectory string
}

// NewBase decodes the shared attributes. A missing name is "default", and a missing directory stays empty (see ResolveDirectories).
func NewBase(spec cty.Value) *Base {
	base := &Base{
		Name:    DefaultGlazeElementName,
		Hooks:   stringMap(spec.GetAttr("hooks")),
		Options: stringMap(spec.GetAttr("options")),
	}

	if name := spec.GetAttr("name"); !name.IsNull() {
		base.Name = name.AsString()
	}

	if directory := spec.GetAttr("starting_directory"); !directory.IsNull() {
		base.StartingDirectory = directory.AsString()
	}

	return base
}

// stringMap returns a map(string) attribute as a Go map, or nil when the attribute is not set.
func stringMap(value cty.Value) map[string]string {
	if value.IsNull() {
		return nil
	}

	out := make(map[string]string, value.LengthInt())
	for key, item := range value.AsValueMap() {
		out[key] = item.AsString()
	}

	return out
}

// stringList returns a list(string) attribute as a Go slice, or nil when the attribute is not set.
func stringList(value cty.Value) []string {
	var out []string
	for _, item := range elements(value) {
		out = append(out, item.AsString())
	}

	return out
}

// elements returns the items of a list attribute or a list of blocks, or nil when it is not set.
func elements(value cty.Value) []cty.Value {
	if value.IsNull() || !value.CanIterateElements() {
		return nil
	}

	return value.AsValueSlice()
}

// isTrue reports whether a bool attribute is set to true. The spec makes the attribute a bool, so no conversion can fail.
func isTrue(value cty.Value) bool {
	return !value.IsNull() && value.True()
}
