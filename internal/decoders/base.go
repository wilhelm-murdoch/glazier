package decoders

import (
	"os"

	"github.com/zclconf/go-cty/cty"
)

const DefaultGlazeElementName = "default"

type Base struct {
	Name              string
	Hooks             map[string]string
	Options           map[string]string
	StartingDirectory string
	Spec              cty.Value
}

func NewBase(spec cty.Value) *Base {
	base := &Base{
		Spec: spec,
	}

	name := spec.GetAttr("name")
	if !name.IsNull() {
		base.Name = name.AsString()
	} else {
		base.Name = DefaultGlazeElementName
	}

	startingDirectory := spec.GetAttr("starting_directory")
	if !startingDirectory.IsNull() {
		base.StartingDirectory = startingDirectory.AsString()
	} else {
		if pwd, err := os.Getwd(); err == nil {
			base.StartingDirectory = pwd
		}
	}

	options := spec.GetAttr("options")
	if !options.IsNull() {
		base.Options = make(map[string]string, len(options.AsValueMap()))
		for name, value := range options.AsValueMap() {
			base.Options[name] = value.AsString()
		}
	}

	hooks := spec.GetAttr("hooks")
	if !hooks.IsNull() {
		base.Hooks = make(map[string]string, len(hooks.AsValueMap()))
		for name, value := range hooks.AsValueMap() {
			base.Hooks[name] = value.AsString()
		}
	}

	return base
}
