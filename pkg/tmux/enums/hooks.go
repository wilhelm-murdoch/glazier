package enums

import (
	"regexp"
	"slices"
)

var (
	// HookList is every hook name, other than an after- hook, that tmux 3.2a to 3.7c know. Some exist only in later versions.
	HookList = []string{
		"alert-activity",
		"alert-bell",
		"alert-silence",
		"client-active",
		"client-attached",
		"client-dark-theme",
		"client-detached",
		"client-focus-in",
		"client-focus-out",
		"client-light-theme",
		"client-resized",
		"client-session-changed",
		"command-error",
		"pane-died",
		"pane-exited",
		"pane-focus-in",
		"pane-focus-out",
		"pane-mode-changed",
		"pane-set-clipboard",
		"pane-title-changed",
		"session-closed",
		"session-created",
		"session-renamed",
		"session-window-changed",
		"window-layout-changed",
		"window-linked",
		"window-pane-changed",
		"window-renamed",
		"window-resized",
		"window-unlinked",
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

// IsHook reports whether some tmux from 3.2a to 3.7c knows the hook name. An index such as [1] may follow the name.
func IsHook(name string) bool {
	name = hookIndexPattern.ReplaceAllString(name, "")

	return slices.Contains(HookList, name) || slices.Contains(AfterHookList, name)
}
