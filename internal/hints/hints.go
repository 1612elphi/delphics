// Package hints reads keybindings from the niri config and groups them for the bar's hint view.
package hints

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Modifier bits, same values as delphics-modd's protocol.
const (
	Super byte = 1 << iota
	Shift
	Ctrl
	Alt
)

type Bind struct {
	Mods   byte
	Key    string
	Action string
	// Dir is set when Action is one of a family of binds that differ only by direction
	// (left, right, up, down, first, last, -, +, or a, b, c for named families); For folds them into one hint.
	Dir string
}

// Hint is one entry in the hint view. Keys are alternatives; a folded family has one key string per
// alternative with the family's keys side by side, e.g. "←→" and "HL".
type Hint struct {
	Keys   []string
	Action string
}

var (
	includeRe = regexp.MustCompile(`^\s*include\s+"([^"]+)"`)
	bindRe    = regexp.MustCompile(`^\s*([A-Za-z0-9_+]+)\b([^{]*)\{\s*(.*?);?\s*\}\s*$`)
	titleRe   = regexp.MustCompile(`hotkey-overlay-title=(?:"([^"]*)"|null)`)
	quotedRe  = regexp.MustCompile(`"([^"]*)"`)
)

// Load returns the binds from a niri config file and its includes, in file order.
// ponytail: only single-line binds are read; multi-line bind blocks in user overrides are skipped
func Load(path string) []Bind {
	var binds []Bind
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	inBinds := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if m := includeRe.FindStringSubmatch(line); m != nil {
			inc := m[1]
			if !filepath.IsAbs(inc) {
				inc = filepath.Join(filepath.Dir(path), inc)
			}
			binds = append(binds, Load(inc)...)
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "binds") && strings.HasSuffix(trimmed, "{") {
			inBinds = true
			continue
		}
		if !inBinds {
			continue
		}
		if trimmed == "}" {
			inBinds = false
			continue
		}
		m := bindRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		b, ok := parseBind(m[1], m[2], m[3])
		if ok {
			binds = append(binds, b)
		}
	}
	return binds
}

func parseBind(combo, props, action string) (Bind, bool) {
	parts := strings.Split(combo, "+")
	var b Bind
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "Mod", "Super", "Win":
			b.Mods |= Super
		case "Shift":
			b.Mods |= Shift
		case "Ctrl", "Control":
			b.Mods |= Ctrl
		case "Alt":
			b.Mods |= Alt
		}
	}
	b.Key = parts[len(parts)-1]
	if strings.HasPrefix(b.Key, "Wheel") || strings.HasPrefix(b.Key, "Touchpad") {
		return b, false
	}
	if t := titleRe.FindStringSubmatch(props); t != nil {
		if t[1] == "" {
			return b, false
		}
		b.Action = t[1]
		return b, true
	}
	b.Action, b.Dir = actionLabel(action)
	return b, true
}

// labels maps niri actions to hint labels of at most two words. Directional actions are keyed without
// their direction: "focus-column" covers focus-column-left/right, "focus-column/ends" covers -first/-last,
// "set-column-width/size" covers both signs. Spawned programs are keyed by binary name.
var labels = map[string]string{
	"focus-column": "column", "focus-window": "window", "focus-monitor": "monitor",
	"focus-column-or-monitor": "column", "focus-window-or-workspace": "window",
	"move-column": "move column", "move-window": "move window",
	"move-column-to-monitor": "to monitor", "move-window-to-monitor": "to monitor",
	"focus-column/ends": "first/last", "move-column-to/ends": "move first/last",
	"focus-column N":        "column N",
	"set-column-width/size": "width", "set-window-height/size": "height",
	"switch-preset-column-width": "width preset", "switch-preset-window-height": "height preset",
	"switch-preset-column-width-back": "width back", "reset-window-height": "reset height",
	"consume-or-expel-window": "push window",
	"focus-window-previous":   "previous", "toggle-overview": "overview",
	"maximize-column": "maximize", "maximize-window-to-edges": "fill screen",
	"expand-column-to-available-width": "expand", "center-column": "center",
	"center-visible-columns": "center all", "toggle-column-tabbed-display": "tabs",
	"close-window": "close", "toggle-keyboard-shortcuts-inhibit": "inhibit keys",
	"fullscreen-window": "fullscreen", "toggle-windowed-fullscreen": "fake fullscreen",
	"toggle-window-floating": "float", "switch-focus-between-floating-and-tiling": "focus float",
	"quit": "quit", "power-off-monitors": "screen off", "show-hotkey-overlay": "hotkeys",
	"wezterm": "terminal", "rofi": "launcher", "cliphist": "clipboard", "helium": "browser",
	"swappy": "annotate", "gtklock": "lock", "darkman": "light/dark", "orca": "screen reader",
}

// families folds related actions that are not named by direction into one hint.
var families = map[string]struct{ label, dir string }{
	"consume-window-into-column": {"join/expel", "a"},
	"expel-window-from-column":   {"join/expel", "b"},
	"screenshot-screen":          {"shot screen/area/window", "a"},
	"screenshot":                 {"shot screen/area/window", "b"},
	"screenshot-window":          {"shot screen/area/window", "c"},
}

var dirOrder = []string{"left", "right", "up", "down", "first", "last", "-", "+", "a", "b", "c"}

func actionLabel(action string) (label, dir string) {
	name, args, _ := strings.Cut(action, " ")
	args = strings.TrimSpace(args)
	if name == "spawn" || name == "spawn-sh" {
		var first string
		for _, q := range quotedRe.FindAllStringSubmatch(args, -1) {
			for _, word := range strings.Fields(q[1]) {
				word = filepath.Base(word)
				if l, ok := labels[word]; ok {
					return l, ""
				}
				if first == "" {
					first = word
				}
			}
		}
		return first, ""
	}
	if f, ok := families[name]; ok {
		return f.label, f.dir
	}
	key := name
	switch {
	case len(args) == 1 && args >= "1" && args <= "9":
		key += " N"
	case strings.HasPrefix(args, `"-`), strings.HasPrefix(args, `"+`):
		key, dir = name+"/size", args[1:2]
	default:
		for _, d := range []string{"left", "right", "up", "down", "first", "last"} {
			if base, ok := strings.CutSuffix(name, "-"+d); ok {
				key, dir = base, d
				if d == "first" || d == "last" {
					key += "/ends"
				}
				break
			}
		}
	}
	if l, ok := labels[key]; ok {
		return l, dir
	}
	// unknown action: its first two words
	words := strings.Split(strings.SplitN(key, "/", 2)[0], "-")
	return strings.Join(words[:min(2, len(words))], " "), dir
}

var keyNames = map[string]string{
	"Left": "←", "Right": "→", "Up": "↑", "Down": "↓",
	"Return": "Enter", "Escape": "Esc", "BracketLeft": "[", "BracketRight": "]",
	"Comma": ",", "Period": ".", "Minus": "-", "Equal": "=", "Slash": "/",
}

// For returns the hints for exactly this modifier combination. A later bind for the same key
// replaces an earlier one, the way niri applies included files.
func For(binds []Bind, mods byte) []Hint {
	byKey := map[string]int{}
	var own []Bind
	for _, b := range binds {
		if b.Mods != mods {
			continue
		}
		if i, ok := byKey[b.Key]; ok {
			own[i] = b
			continue
		}
		byKey[b.Key] = len(own)
		own = append(own, b)
	}

	// keys per action, then per direction, in config order
	var actions []string
	keys := map[string]map[string][]string{}
	for _, b := range own {
		key := b.Key
		if len(key) == 1 && key >= "1" && key <= "9" && strings.HasSuffix(b.Action, " N") {
			key = "1-9"
		} else if name, ok := keyNames[key]; ok {
			key = name
		}
		if keys[b.Action] == nil {
			keys[b.Action] = map[string][]string{}
			actions = append(actions, b.Action)
		}
		if !contains(keys[b.Action][b.Dir], key) {
			keys[b.Action][b.Dir] = append(keys[b.Action][b.Dir], key)
		}
	}

	var out []Hint
	for _, action := range actions {
		byDir := keys[action]
		if plain := byDir[""]; plain != nil {
			out = append(out, Hint{Keys: plain, Action: action})
		}
		if folded := fold(byDir); folded != nil {
			out = append(out, Hint{Keys: folded, Action: action})
		}
	}
	return out
}

// fold puts the i-th key of every direction side by side: left ←/H and right →/L become ←→ and HL.
func fold(byDir map[string][]string) []string {
	var out []string
	for i := 0; ; i++ {
		var parts []string
		short := true
		for _, d := range dirOrder {
			if ks := byDir[d]; i < len(ks) {
				parts = append(parts, ks[i])
				short = short && utf8.RuneCountInString(ks[i]) == 1
			}
		}
		if parts == nil {
			return out
		}
		sep := " "
		if short {
			sep = ""
		}
		out = append(out, strings.Join(parts, sep))
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
