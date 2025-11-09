package session

import (
	"github.com/wilhelm-murdoch/go-collection"
	"github.com/zclconf/go-cty/cty"

	"github.com/wilhelm-murdoch/glazier/internal/decoders"
	"github.com/wilhelm-murdoch/glazier/internal/decoders/window"
)

type Session struct {
	*decoders.BaseDecoder
	Windows  collection.Collection[*window.Window]
	Commands []string
}

func New(spec cty.Value) *Session {
	base := decoders.New(spec)

	return &Session{
		BaseDecoder: base,
	}
}
