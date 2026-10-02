// Package audio reads and sets PipeWire volume through wpctl and follows changes through pw-dump --monitor.
package audio

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

const (
	sink   = "@DEFAULT_AUDIO_SINK@"
	source = "@DEFAULT_AUDIO_SOURCE@"
)

type Device struct {
	ID          int
	Description string
	Default     bool
}

type State struct {
	HasOutput   bool
	Volume      float64
	Muted       bool
	HasInput    bool
	InputVolume float64
	InputMuted  bool
	Outputs     []Device
	Inputs      []Device
}

var volumeRe = regexp.MustCompile(`^Volume: ([0-9.]+)( \[MUTED\])?`)

// parseVolume reads wpctl get-volume output, e.g. "Volume: 0.40 [MUTED]".
func parseVolume(out string) (vol float64, muted, ok bool) {
	m := volumeRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false, false
	}
	vol, err := strconv.ParseFloat(m[1], 64)
	return vol, m[2] != "", err == nil
}

func getVolume(node string) (float64, bool, bool) {
	out, err := exec.Command("wpctl", "get-volume", node).Output()
	if err != nil {
		return 0, false, false
	}
	return parseVolume(string(out))
}

// Read returns the default output and input and the devices to choose from.
func Read() State {
	var s State
	s.Volume, s.Muted, s.HasOutput = getVolume(sink)
	s.InputVolume, s.InputMuted, s.HasInput = getVolume(source)
	if out, err := exec.Command("pw-dump").Output(); err == nil {
		s.Outputs, s.Inputs = parseDevices(out)
	}
	return s
}

// parseDevices finds audio sinks and sources and the defaults in pw-dump's JSON.
func parseDevices(dump []byte) (outputs, inputs []Device) {
	var objects []struct {
		ID    int    `json:"id"`
		Type  string `json:"type"`
		Props struct {
			Name string `json:"metadata.name"`
		} `json:"props"`
		Info struct {
			Props struct {
				Class       string `json:"media.class"`
				Name        string `json:"node.name"`
				Description string `json:"node.description"`
			} `json:"props"`
		} `json:"info"`
		Metadata []struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(dump, &objects); err != nil {
		return nil, nil
	}
	defaults := map[string]string{}
	for _, o := range objects {
		if o.Type != "PipeWire:Interface:Metadata" || o.Props.Name != "default" {
			continue
		}
		for _, m := range o.Metadata {
			var v struct{ Name string }
			if json.Unmarshal(m.Value, &v) == nil {
				defaults[m.Key] = v.Name
			}
		}
	}
	for _, o := range objects {
		p := o.Info.Props
		d := Device{ID: o.ID, Description: p.Description}
		switch p.Class {
		case "Audio/Sink":
			d.Default = p.Name == defaults["default.audio.sink"]
			outputs = append(outputs, d)
		case "Audio/Source":
			d.Default = p.Name == defaults["default.audio.source"]
			inputs = append(inputs, d)
		}
	}
	return outputs, inputs
}

func node(input bool) string {
	if input {
		return source
	}
	return sink
}

// SetVolume sets the default output or input volume, 0 to 1.
func SetVolume(input bool, v float64) error {
	return exec.Command("wpctl", "set-volume", node(input), fmt.Sprintf("%.2f", min(max(v, 0), 1))).Run()
}

func ToggleMute(input bool) error {
	return exec.Command("wpctl", "set-mute", node(input), "toggle").Run()
}

func SetDefault(id int) error {
	return exec.Command("wpctl", "set-default", strconv.Itoa(id)).Run()
}

// debounce collects a burst of PipeWire changes into one read.
const debounce = 150 * time.Millisecond

// Watch calls onChange after PipeWire objects change. It runs pw-dump --monitor and restarts it if it exits.
func Watch(onChange func()) {
	go func() {
		for {
			if err := monitor(onChange); err != nil {
				log.Printf("pw-dump: %v", err)
			}
			time.Sleep(2 * time.Second)
			onChange()
		}
	}()
}

func monitor(onChange func()) error {
	cmd := exec.Command("pw-dump", "--monitor", "--no-colors")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var timer *time.Timer
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		if timer == nil {
			timer = time.AfterFunc(debounce, onChange)
		} else {
			timer.Reset(debounce)
		}
	}
	return cmd.Wait()
}
