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
    Mod+1 { focus-column 1; }
    Mod+2 { focus-column 2; }
    Mod+Return { spawn "wezterm"; }
    Mod+V repeat=false { spawn-sh "cliphist list | rofi -dmenu | cliphist decode | wl-copy"; }
    Mod+Shift+H { move-column-left; }
    Mod+Minus { set-column-width "-10%"; }
    Mod+WheelScrollRight { focus-column-right; }
    Mod+Alt+S hotkey-overlay-title="Screen reader" { spawn-sh "pkill orca || exec orca"; }
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
		{Keys: []string{"←", "H"}, Action: "focus column left"},
		{Keys: []string{"1-9"}, Action: "focus column N"},
		{Keys: []string{"Enter"}, Action: "foot"},
		{Keys: []string{"V"}, Action: "cliphist"},
		{Keys: []string{"-"}, Action: "set column width -10%"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Super hints:\n got %+v\nwant %+v", got, want)
	}
	if got := For(binds, Super|Shift); len(got) != 1 || got[0].Action != "move column left" {
		t.Fatalf("Super+Shift hints: %+v", got)
	}
	if got := For(binds, Super|Alt); len(got) != 1 || got[0].Action != "Screen reader" {
		t.Fatalf("Super+Alt hints: %+v", got)
	}
}
