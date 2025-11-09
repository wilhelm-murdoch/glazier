package schema

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type BaseSchema struct {
	Name              string
	Envs              map[string]string
	Hooks             map[string]string
	Options           map[string]string
	StartingDirectory string
}

func New() (*BaseSchema, hcl.Diagnostics) {
	return &BaseSchema{}, nil
}

func (b *BaseSchema) DecodeEnvs(envs cty.Value) map[string]string {
	out := make(map[string]string, len(envs.AsValueMap()))
	for name, value := range envs.AsValueMap() {
		out[name] = value.AsString()
	}

	return out
}

func (b *BaseSchema) DecodeCommands(commands cty.Value) []string {
	var out []string

	if commands.CanIterateElements() {
		commandIterator := commands.ElementIterator()

		for commandIterator.Next() {
			_, command := commandIterator.Element()
			if command.Type().FriendlyName() == "string" {
				out = append(out, command.AsString())
			}
		}
	}

	return out
}
