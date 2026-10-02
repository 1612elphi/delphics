package backlight

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPrefersFirmwareAndReadsLevel(t *testing.T) {
	root := t.TempDir()
	for name, typ := range map[string]string{"acpi_video0": "firmware", "intel_backlight": "raw"} {
		d := filepath.Join(root, name)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "type"), []byte(typ+"\n"), 0o644)
		os.WriteFile(filepath.Join(d, "brightness"), []byte("30\n"), 0o644)
		os.WriteFile(filepath.Join(d, "max_brightness"), []byte("120\n"), 0o644)
	}
	b, err := find(root)
	if err != nil {
		t.Skip(err) // no system bus
	}
	if b == nil || filepath.Base(b.dir) != "acpi_video0" {
		t.Fatalf("picked %+v, want acpi_video0", b)
	}
	if l, err := b.Level(); err != nil || l != 0.25 {
		t.Errorf("level %v %v, want 0.25", l, err)
	}
	if b, _ := find(t.TempDir()); b != nil {
		t.Error("found a backlight in an empty directory")
	}
}
