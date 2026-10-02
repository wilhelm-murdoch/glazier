package enums

import (
	"regexp"
	"slices"
)

type Hook int

const (
	HookAlertActivity Hook = iota + 1
	HookAlertBell
	HookAlertSilence
	HookClientActive
	HookClientAttached
	HookClientDarkTheme
	HookClientDetached
	HookClientFocusIn
	HookClientFocusOut
	HookClientLightTheme
	HookClientResized
	HookClientSessionChanged
	HookCommandError
	HookPaneDied
	HookPaneExited
	HookPaneFocusIn
	HookPaneFocusOut
	HookPaneModeChanged
	HookPaneSetClipboard
	HookPaneTitleChanged
	HookSessionClosed
	HookSessionCreated
	HookSessionRenamed
	HookSessionWindowChanged
	HookWindowLayoutChanged
	HookWindowLinked
	HookWindowPaneChanged
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
	HookClientDarkThemeString      = "client-dark-theme"
	HookClientDetachedString       = "client-detached"
	HookClientFocusInString        = "client-focus-in"
	HookClientFocusOutString       = "client-focus-out"
	HookClientLightThemeString     = "client-light-theme"
	HookClientResizedString        = "client-resized"
	HookClientSessionChangedString = "client-session-changed"
	HookCommandErrorString         = "command-error"
	HookPaneDiedString             = "pane-died"
	HookPaneExitedString           = "pane-exited"
	HookPaneFocusInString          = "pane-focus-in"
	HookPaneFocusOutString         = "pane-focus-out"
	HookPaneModeChangedString      = "pane-mode-changed"
	HookPaneSetClipboardString     = "pane-set-clipboard"
	HookPaneTitleChangedString     = "pane-title-changed"
	HookSessionClosedString        = "session-closed"
	HookSessionCreatedString       = "session-created"
	HookSessionRenamedString       = "session-renamed"
	HookSessionWindowChangedString = "session-window-changed"
	HookWindowLayoutChangedString  = "window-layout-changed"
	HookWindowLinkedString         = "window-linked"
	HookWindowPaneChangedString    = "window-pane-changed"
	HookWindowRenamedString        = "window-renamed"
	HookWindowResizedString        = "window-resized"
	HookWindowUnlinkedString       = "window-unlinked"
	HookUnknownString              = "unknown"
)

var (
	// HookList is every hook name, other than an after- hook, that tmux 3.2a to 3.7c know. Some exist only in later versions.
	HookList = []string{
		HookAlertActivityString,
		HookAlertBellString,
		HookAlertSilenceString,
		HookClientActiveString,
		HookClientAttachedString,
		HookClientDarkThemeString,
		HookClientDetachedString,
		HookClientFocusInString,
		HookClientFocusOutString,
		HookClientLightThemeString,
		HookClientResizedString,
		HookClientSessionChangedString,
		HookCommandErrorString,
		HookPaneDiedString,
		HookPaneExitedString,
		HookPaneFocusInString,
		HookPaneFocusOutString,
		HookPaneModeChangedString,
		HookPaneSetClipboardString,
		HookPaneTitleChangedString,
		HookSessionClosedString,
		HookSessionCreatedString,
		HookSessionRenamedString,
		HookSessionWindowChangedString,
		HookWindowLayoutChangedString,
		HookWindowLinkedString,
		HookWindowPaneChangedString,
		HookWindowRenamedString,
		HookWindowResizedString,
		HookWindowUnlinkedString,
	}

	// AfterHookList is every after- hook that tmux 3.2a to 3.7c know: one for each command that has one.
	AfterHookList = []string{
		"after-bind-key",
		"after-capture-pane",
		"after-copy-mode",
		"after-display-message",
		"after-display-panes",
		"after-kill-pane",
		"after-list-buffers",
		"after-list-clients",
		"after-list-keys",
		"after-list-panes",
		"after-list-sessions",
		"after-list-windows",
		"after-load-buffer",
		"after-lock-server",
		"after-new-session",
		"after-new-window",
		"after-paste-buffer",
		"after-pipe-pane",
		"after-queue",
		"after-refresh-client",
		"after-rename-session",
		"after-rename-window",
		"after-resize-pane",
		"after-resize-window",
		"after-save-buffer",
		"after-select-layout",
		"after-select-pane",
		"after-select-window",
		"after-send-keys",
		"after-set-buffer",
		"after-set-environment",
		"after-set-hook",
		"after-set-option",
		"after-show-environment",
		"after-show-messages",
		"after-show-options",
		"after-split-window",
		"after-unbind-key",
	}

	// hookIndexPattern matches the array index that tmux allows after a hook name, for example session-created[1].
	hookIndexPattern = regexp.MustCompile(`\[[0-9]+\]$`)
)

// String returns the tmux name of the hook.
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
	case HookClientDarkTheme:
		return HookClientDarkThemeString
	case HookClientDetached:
		return HookClientDetachedString
	case HookClientFocusIn:
		return HookClientFocusInString
	case HookClientFocusOut:
		return HookClientFocusOutString
	case HookClientLightTheme:
		return HookClientLightThemeString
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
	case HookPaneModeChanged:
		return HookPaneModeChangedString
	case HookPaneSetClipboard:
		return HookPaneSetClipboardString
	case HookPaneTitleChanged:
		return HookPaneTitleChangedString
	case HookSessionClosed:
		return HookSessionClosedString
	case HookSessionCreated:
		return HookSessionCreatedString
	case HookSessionRenamed:
		return HookSessionRenamedString
	case HookSessionWindowChanged:
		return HookSessionWindowChangedString
	case HookWindowLayoutChanged:
		return HookWindowLayoutChangedString
	case HookWindowLinked:
		return HookWindowLinkedString
	case HookWindowPaneChanged:
		return HookWindowPaneChangedString
	case HookWindowRenamed:
		return HookWindowRenamedString
	case HookWindowResized:
		return HookWindowResizedString
	case HookWindowUnlinked:
		return HookWindowUnlinkedString
	}

	return HookUnknownString
}

// HookFromString returns the hook with the tmux name s.
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
	case HookClientDarkThemeString:
		return HookClientDarkTheme
	case HookClientDetachedString:
		return HookClientDetached
	case HookClientFocusInString:
		return HookClientFocusIn
	case HookClientFocusOutString:
		return HookClientFocusOut
	case HookClientLightThemeString:
		return HookClientLightTheme
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
	case HookPaneModeChangedString:
		return HookPaneModeChanged
	case HookPaneSetClipboardString:
		return HookPaneSetClipboard
	case HookPaneTitleChangedString:
		return HookPaneTitleChanged
	case HookSessionClosedString:
		return HookSessionClosed
	case HookSessionCreatedString:
		return HookSessionCreated
	case HookSessionRenamedString:
		return HookSessionRenamed
	case HookSessionWindowChangedString:
		return HookSessionWindowChanged
	case HookWindowLayoutChangedString:
		return HookWindowLayoutChanged
	case HookWindowLinkedString:
		return HookWindowLinked
	case HookWindowPaneChangedString:
		return HookWindowPaneChanged
	case HookWindowRenamedString:
		return HookWindowRenamed
	case HookWindowResizedString:
		return HookWindowResized
	case HookWindowUnlinkedString:
		return HookWindowUnlinked
	}

	return HookUnknown
}

// IsHook reports whether some tmux from 3.2a to 3.7c knows the hook name. An index such as [1] may follow the name.
func IsHook(name string) bool {
	name = hookIndexPattern.ReplaceAllString(name, "")

	return slices.Contains(HookList, name) || slices.Contains(AfterHookList, name)
}
