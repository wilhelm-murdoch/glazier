package session

import (
	"github.com/hashicorp/hcl/v2"

	"github.com/wilhelm-murdoch/glazier/internal/decoders/window"
)

// Decode is responsible for decoding a cty.Value into a Session struct.
func (s *Session) Decode() hcl.Diagnostics {
	var diags hcl.Diagnostics

	windows := s.Spec.GetAttr("windows")
	if windows.CanIterateElements() {
		windowIterator := windows.ElementIterator()

		for windowIterator.Next() {
			_, spec := windowIterator.Element()

			window := window.New(spec)
			if diags = window.Decode(); diags.HasErrors() {
				diags = diags.Extend(diags)
				continue
			}

			s.Windows.Push(window)
		}
	}

	return diags
}
