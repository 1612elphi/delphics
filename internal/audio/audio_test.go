package audio

import "testing"

func TestParseVolume(t *testing.T) {
	for _, tc := range []struct {
		out   string
		vol   float64
		muted bool
		ok    bool
	}{
		{"Volume: 0.40 [MUTED]\n", 0.40, true, true},
		{"Volume: 1.00\n", 1, false, true},
		{"Translate ID error: '@DEFAULT_AUDIO_SINK@' is not a valid ID\n", 0, false, false},
	} {
		vol, muted, ok := parseVolume(tc.out)
		if vol != tc.vol || muted != tc.muted || ok != tc.ok {
			t.Errorf("%q: got %v %v %v", tc.out, vol, muted, ok)
		}
	}
}

func TestParseDevices(t *testing.T) {
	dump := []byte(`[
	 {"id": 30, "type": "PipeWire:Interface:Metadata", "props": {"metadata.name": "default"},
	  "metadata": [{"key": "default.audio.sink", "value": {"name": "hdmi"}},
	               {"key": "default.audio.source", "value": {"name": "mic"}}]},
	 {"id": 55, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "speakers", "node.description": "Speakers"}}},
	 {"id": 57, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Sink", "node.name": "hdmi", "node.description": "HDMI"}}},
	 {"id": 56, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Audio/Source", "node.name": "mic", "node.description": "Mic"}}},
	 {"id": 60, "type": "PipeWire:Interface:Node", "info": {"props": {"media.class": "Stream/Output/Audio", "node.name": "firefox"}}}
	]`)
	outs, ins := parseDevices(dump)
	if len(outs) != 2 || outs[0].Default || !outs[1].Default || outs[1].Description != "HDMI" {
		t.Errorf("outputs %+v", outs)
	}
	if len(ins) != 1 || !ins[0].Default || ins[0].ID != 56 {
		t.Errorf("inputs %+v", ins)
	}
}
