package schema

import "github.com/zclconf/go-cty/cty"

type Base struct {
	Name  Name
	Envs  Envs
	Hooks Hooks
}

func (b *Base) DecodeEnvs(envs cty.Value) Envs {
	out := make(Envs, len(envs.AsValueMap()))
	for name, value := range envs.AsValueMap() {
		out[Name(name)] = Value(value.AsString())
	}

	return out
}

func (b *Base) DecodeCommands(commands cty.Value) Commands {
	var out Commands

	if commands.CanIterateElements() {
		commandIterator := commands.ElementIterator()

		for commandIterator.Next() {
			_, command := commandIterator.Element()
			if command.Type().FriendlyName() == "string" {
				out = append(out, Command(command.AsString()))
			}
		}
	}

	return out
}
