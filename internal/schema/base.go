package schema

import (
	"os"

	"github.com/zclconf/go-cty/cty"
)

const DefaultGlazeElementName = "default"

type BaseSchema struct {
	Name              string
	Envs              map[string]string
	Hooks             map[string]string
	Options           map[string]string
	StartingDirectory string
	Spec              cty.Value
}

func New(spec cty.Value) *BaseSchema {
	base := &BaseSchema{
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

	envs := spec.GetAttr("envs")
	if !envs.IsNull() {
		base.Envs = make(map[string]string, len(envs.AsValueMap()))
		for name, value := range envs.AsValueMap() {
			base.Envs[name] = value.AsString()
		}
	}

	// TODO: Implement support for modifying session, window and pane options
	// options := spec.GetAttr("options")
	// if !options.IsNull() {
	// 	base.Options = make(map[string]string, len(options.AsValueMap()))
	// 	for name, value := range options.AsValueMap() {
	// 		base.Options[name] = value.AsString()
	// 	}
	// }

	hooks := spec.GetAttr("hooks")
	if !hooks.IsNull() {
		base.Hooks = make(map[string]string, len(hooks.AsValueMap()))
		for name, value := range hooks.AsValueMap() {
			base.Hooks[name] = value.AsString()
		}
	}

	return base
}
