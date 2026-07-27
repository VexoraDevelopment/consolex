package cmdline

import (
	"reflect"
	"testing"
)

func TestAvailableCommandInfosPreservesLegacyNamesAndAliases(t *testing.T) {
	loop := NewLoop(Options{
		CommandNames: func(string) []string { return []string{"remote", "REMOTE"} },
		CommandInfos: func(string) []CommandInfo {
			return []CommandInfo{{Name: "described", Description: "dynamic command"}}
		},
	})
	loop.Register(Command{Name: "local", Aliases: []string{"l"}, Description: "local command"})

	got := loop.availableCommandInfos("")
	want := []CommandInfo{
		{Name: "clear", Description: "Clear terminal screen"},
		{Name: "cls", Description: "Clear terminal screen"},
		{Name: "described", Description: "dynamic command"},
		{Name: "echo", Description: "Print text back to console"},
		{Name: "help", Description: "Show available console commands"},
		{Name: "l", Description: "local command"},
		{Name: "local", Description: "local command"},
		{Name: "pid", Description: "Show current process id"},
		{Name: "remote"},
		{Name: "uptime", Description: "Show console loop uptime"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("available command infos:\ngot  %#v\nwant %#v", got, want)
	}
}
