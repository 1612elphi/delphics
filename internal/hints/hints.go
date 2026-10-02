// Package hints reads keybindings from the niri config and groups them for the bar's hint view.
package hints

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
}

// Hint is one entry in the hint view; Keys has every key bound to the same action.
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
	b.Action = actionLabel(action)
	return b, true
}

func actionLabel(action string) string {
	name, args, _ := strings.Cut(action, " ")
	if name == "spawn" || name == "spawn-sh" {
		if q := quotedRe.FindStringSubmatch(args); q != nil {
			return filepath.Base(strings.Fields(q[1])[0])
		}
	}
	label := strings.ReplaceAll(name, "-", " ")
	if args = strings.TrimSpace(strings.ReplaceAll(args, `"`, "")); args != "" {
		label += " " + args
	}
	return label
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

	var out []Hint
	index := map[string]int{}
	for _, b := range own {
		key, action := b.Key, b.Action
		if n := len(key); n == 1 && key >= "1" && key <= "9" && strings.HasSuffix(action, " "+key) {
			action = strings.TrimSuffix(action, key) + "N"
			key = "1-9"
		} else if name, ok := keyNames[key]; ok {
			key = name
		}
		if i, ok := index[action]; ok {
			if !contains(out[i].Keys, key) {
				out[i].Keys = append(out[i].Keys, key)
			}
			continue
		}
		index[action] = len(out)
		out = append(out, Hint{Keys: []string{key}, Action: action})
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
