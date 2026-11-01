package main

import (
	"regexp"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// commandNameRE is what Telegram accepts as a command name: 1-32 characters of
// a-z, 0-9 and underscore.
var commandNameRE = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// TestCommandList checks the menu the bot registers at startup. Telegram
// refuses the whole setMyCommands call when one entry is wrong (a 33-character
// name, a duplicate, an empty description), and the bot only logs that, so the
// command menu silently stops updating. The menu is also capped at 100 commands
// per scope.
func TestCommandList(t *testing.T) {
	commands := commandListConfig()

	seen := map[string]int{}
	for i, entry := range commands {
		name := entry.command.Command

		if !commandNameRE.MatchString(name) {
			t.Errorf("command %d = %q, want 1-32 characters of a-z, 0-9 and _", i, name)
		}
		if len(name) > 32 {
			t.Errorf("command %q is %d characters, Telegram allows 32", name, len(name))
		}
		if entry.command.Description == "" {
			t.Errorf("command %q has no description", name)
		}
		if runes := len([]rune(entry.command.Description)); runes > 256 {
			t.Errorf("command %q description is %d characters, Telegram allows 256", name, runes)
		}
		if entry.response == nil {
			t.Errorf("command %q has no handler", name)
		}

		seen[name]++
	}

	for name, count := range seen {
		if count > 1 {
			t.Errorf("command %q is listed %d times, want it once", name, count)
		}
	}

	group, private, admin := commandMenus(commands)
	for _, menu := range [][]gotgbot.BotCommand{group, private, admin} {
		if len(menu) > 100 {
			t.Errorf("the menu has %d commands, Telegram allows 100 per scope", len(menu))
		}
	}
	if len(commands) == 0 {
		t.Errorf("the menu is empty")
	}
}
