package visualtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validDef() Definition {
	return Definition{
		ID: "chat.reply.streaming", Description: "Pending assistant reply", Fixture: "streaming-reply",
		Checkpoints: []string{"waiting", "partial"}, Viewports: []string{"desktop", "mobile"}, Themes: []string{"light", "dark"},
	}
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*Definition)
		good   bool
	}{
		{"valid", func(*Definition) {}, true},
		{"bad ID", func(d *Definition) { d.ID = "Chat.reply.streaming" }, false},
		{"bad description", func(d *Definition) { d.Description = "  " }, false},
		{"bad fixture", func(d *Definition) { d.Fixture = "fixtures/streaming" }, false},
		{"duplicate checkpoint", func(d *Definition) { d.Checkpoints = []string{"waiting", "waiting"} }, false},
		{"unknown theme", func(d *Definition) { d.Themes = []string{"orange"} }, false},
		{"bad viewport", func(d *Definition) { d.Viewports = []string{"mobile!"} }, false},
		{"no checkpoints", func(d *Definition) { d.Checkpoints = nil }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := validDef()
			tc.modify(&d)
			if got := d.Validate() == nil; got != tc.good {
				t.Fatalf("valid=%v want %v", got, tc.good)
			}
		})
	}
}
func TestLoad(t *testing.T) {
	dir := t.TempDir()
	data := `{"id":"chat.reply.streaming","description":"Pending","fixture":"streaming-reply","checkpoints":["waiting"],"viewports":["desktop"],"themes":["dark"]}`
	name := filepath.Join(dir, "chat.reply.streaming.json")
	if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	defs, err := Load(dir)
	if err != nil || len(defs) != 1 {
		t.Fatalf("load: %v %+v", err, defs)
	}
	one := defs[0].Fingerprint()
	if err := os.WriteFile(name, []byte("  "+strings.ReplaceAll(data, `"description":"Pending"`, `"description": "Pending"`)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	defs, err = Load(dir)
	if err != nil || defs[0].Fingerprint() != one {
		t.Fatalf("formatting changed contract: %v", err)
	}
	if err := os.WriteFile(name, []byte(data+data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
	if err := os.WriteFile(name, []byte(strings.Replace(data, `"description":`, `"unknown":true,"description":`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("unknown JSON field accepted")
	}
}
func TestFilename(t *testing.T) {
	got, err := Filename("chat.reply.streaming", "waiting", "mobile", "dark")
	if err != nil || got != "chat.reply.streaming--waiting--mobile--dark.png" {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := Filename("../../etc/passwd", "waiting", "mobile", "dark"); err == nil {
		t.Fatal("unsafe filename accepted")
	}
}
