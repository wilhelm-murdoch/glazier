package session

import (
	"github.com/wilhelm-murdoch/glazier/internal/schema"
	"github.com/wilhelm-murdoch/glazier/internal/schema/window"
	"github.com/wilhelm-murdoch/go-collection"
	"github.com/zclconf/go-cty/cty"
)

type Session struct {
	*schema.BaseSchema
	Windows  collection.Collection[*window.Window]
	Commands []string
}

func New(spec cty.Value) *Session {
	base := schema.New(spec)

	return &Session{
		BaseSchema: base,
	}
}
