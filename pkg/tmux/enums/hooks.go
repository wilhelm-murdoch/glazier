package enums

type Hook int

const (
	HookAlertActivity Hook = iota + 1
	HookAlertBell
	HookAlertSilence
	HookClientActive
	HookClientAttached
	HookClientDetached
	HookClientFocusIn
	HookClientFocusOut
	HookClientResized
	HookClientSessionChanged
	HookCommandError
	HookPaneDied
	HookPaneExited
	HookPaneFocusIn
	HookPaneFocusOut
	HookPaneSetClipboard
	HookSessionCreated
	HookSessionClosed
	HookSessionRenamed
	HookWindowLinked
	HookWindowRenamed
	HookWindowResized
	HookWindowUnlinked
	HookUnknown
)

const (
	HookAlertActivityString        = "alert-activity"
	HookAlertBellString            = "alert-bell"
	HookAlertSilenceString         = "alert-silence"
	HookClientActiveString         = "client-active"
	HookClientAttachedString       = "client-attached"
	HookClientDetachedString       = "client-detached"
	HookClientFocusInString        = "client-focus-in"
	HookClientFocusOutString       = "client-focus-out"
	HookClientResizedString        = "client-resized"
	HookClientSessionChangedString = "client-session-changed"
	HookCommandErrorString         = "command-error"
	HookPaneDiedString             = "pane-died"
	HookPaneExitedString           = "pane-exited"
	HookPaneFocusInString          = "pane-focus-in"
	HookPaneFocusOutString         = "pane-focus-out"
	HookPaneSetClipboardString     = "pane-set-clipboard"
	HookSessionCreatedString       = "session-created"
	HookSessionClosedString        = "session-closed"
	HookSessionRenamedString       = "session-renamed"
	HookWindowLinkedString         = "window-linked"
	HookWindowRenamedString        = "window-renamed"
	HookWindowResizedString        = "window-resized"
	HookWindowUnlinkedString       = "window-unlinked"
	HookUnknownString              = "unknown"
)

var HookList = []string{
	HookAlertActivityString,
	HookAlertBellString,
	HookAlertSilenceString,
	HookClientActiveString,
	HookClientAttachedString,
	HookClientDetachedString,
	HookClientFocusInString,
	HookClientFocusOutString,
	HookClientResizedString,
	HookClientSessionChangedString,
	HookCommandErrorString,
	HookPaneDiedString,
	HookPaneExitedString,
	HookPaneFocusInString,
	HookPaneFocusOutString,
	HookPaneSetClipboardString,
	HookSessionCreatedString,
	HookSessionClosedString,
	HookSessionRenamedString,
	HookWindowLinkedString,
	HookWindowRenamedString,
	HookWindowResizedString,
	HookWindowUnlinkedString,
}

// String is responsible for returning the string representation of a Hook.
func (h Hook) String() string {
	switch h {
	case HookAlertActivity:
		return HookAlertActivityString
	case HookAlertBell:
		return HookAlertBellString
	case HookAlertSilence:
		return HookAlertSilenceString
	case HookClientActive:
		return HookClientActiveString
	case HookClientAttached:
		return HookClientAttachedString
	case HookClientDetached:
		return HookClientDetachedString
	case HookClientFocusIn:
		return HookClientFocusInString
	case HookClientFocusOut:
		return HookClientFocusOutString
	case HookClientResized:
		return HookClientResizedString
	case HookClientSessionChanged:
		return HookClientSessionChangedString
	case HookCommandError:
		return HookCommandErrorString
	case HookPaneDied:
		return HookPaneDiedString
	case HookPaneExited:
		return HookPaneExitedString
	case HookPaneFocusIn:
		return HookPaneFocusInString
	case HookPaneFocusOut:
		return HookPaneFocusOutString
	case HookPaneSetClipboard:
		return HookPaneSetClipboardString
	case HookSessionCreated:
		return HookSessionCreatedString
	case HookSessionClosed:
		return HookSessionClosedString
	case HookSessionRenamed:
		return HookSessionRenamedString
	case HookWindowLinked:
		return HookWindowLinkedString
	case HookWindowRenamed:
		return HookWindowRenamedString
	case HookWindowResized:
		return HookWindowResizedString
	case HookWindowUnlinked:
		return HookWindowUnlinkedString
	}

	return HookUnknownString
}

// HookFromString is responsible for converting a string to a Hook enum.
func HookFromString(s string) Hook {
	switch s {
	case HookAlertActivityString:
		return HookAlertActivity
	case HookAlertBellString:
		return HookAlertBell
	case HookAlertSilenceString:
		return HookAlertSilence
	case HookClientActiveString:
		return HookClientActive
	case HookClientAttachedString:
		return HookClientAttached
	case HookClientDetachedString:
		return HookClientDetached
	case HookClientFocusInString:
		return HookClientFocusIn
	case HookClientFocusOutString:
		return HookClientFocusOut
	case HookClientResizedString:
		return HookClientResized
	case HookClientSessionChangedString:
		return HookClientSessionChanged
	case HookCommandErrorString:
		return HookCommandError
	case HookPaneDiedString:
		return HookPaneDied
	case HookPaneExitedString:
		return HookPaneExited
	case HookPaneFocusInString:
		return HookPaneFocusIn
	case HookPaneFocusOutString:
		return HookPaneFocusOut
	case HookPaneSetClipboardString:
		return HookPaneSetClipboard
	case HookSessionCreatedString:
		return HookSessionCreated
	case HookSessionClosedString:
		return HookSessionClosed
	case HookSessionRenamedString:
		return HookSessionRenamed
	case HookWindowLinkedString:
		return HookWindowLinked
	case HookWindowRenamedString:
		return HookWindowRenamed
	case HookWindowResizedString:
		return HookWindowResized
	case HookWindowUnlinkedString:
		return HookWindowUnlinked
	}

	return HookUnknown
}
