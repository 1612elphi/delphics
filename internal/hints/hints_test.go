package hints

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestForGroupsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	system := `
layout { gaps 8; }
binds {
    Mod+Left  { focus-column-left; }
    Mod+H     { focus-column-left; }
    Mod+Right { focus-column-right; }
    Mod+L     { focus-column-right; }
    Mod+Home  { focus-column-first; }
    Mod+End   { focus-column-last; }
    Mod+1 { focus-column 1; }
    Mod+2 { focus-column 2; }
    Mod+Return { spawn "wezterm"; }
    Mod+V repeat=false { spawn-sh "cliphist list | rofi -dmenu | cliphist decode | wl-copy"; }
    Mod+Shift+H { move-column-left; }
    Mod+Minus { set-column-width "-10%"; }
    Mod+Equal { set-column-width "+10%"; }
    Mod+Comma  { consume-window-into-column; }
    Mod+Period { expel-window-from-column; }
    Mod+X { some-unknown-new-action; }
    Mod+WheelScrollRight { focus-column-right; }
    Mod+Alt+S { spawn-sh "pkill orca || exec orca"; }
    Mod+Alt+P hotkey-overlay-title="Screen reader tool" { spawn "foo"; }
    Mod+Alt+Left { focus-monitor-left; }
    Mod+Alt+Right { focus-monitor-right; }
    Mod+Alt+Up { focus-monitor-up; }
    Mod+Alt+Down { focus-monitor-down; }
    Mod+O hotkey-overlay-title=null { toggle-overview; }
}
`
	user := `include "/SYSTEM"
binds {
    Mod+Return { spawn "foot"; }
}
`
	sysPath := filepath.Join(dir, "system.kdl")
	userPath := filepath.Join(dir, "config.kdl")
	os.WriteFile(sysPath, []byte(system), 0o644)
	os.WriteFile(userPath, []byte(strings.ReplaceAll(user, "/SYSTEM", sysPath)), 0o644)

	binds := Load(userPath)
	got := For(binds, Super)
	want := []Hint{
		{Keys: []string{"←→", "HL"}, Action: "column"},
		{Keys: []string{"Home End"}, Action: "first/last"},
		{Keys: []string{"1-9"}, Action: "column N"},
		{Keys: []string{"Enter"}, Action: "foot"},
		{Keys: []string{"V"}, Action: "clipboard"},
		{Keys: []string{"-="}, Action: "width"},
		{Keys: []string{",."}, Action: "join/expel"},
		{Keys: []string{"X"}, Action: "some unknown"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Super hints:\n got %+v\nwant %+v", got, want)
	}
	if got := For(binds, Super|Shift); len(got) != 1 || got[0].Action != "move column" || got[0].Keys[0] != "H" {
		t.Fatalf("Super+Shift hints: %+v", got)
	}
	want = []Hint{
		{Keys: []string{"S"}, Action: "screen reader"},
		{Keys: []string{"P"}, Action: "Screen reader tool"},
		{Keys: []string{"←→↑↓"}, Action: "monitor"},
	}
	if got := For(binds, Super|Alt); !reflect.DeepEqual(got, want) {
		t.Fatalf("Super+Alt hints:\n got %+v\nwant %+v", got, want)
	}
}
