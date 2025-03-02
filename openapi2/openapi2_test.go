package openapi2

import (
	"bytes"
	"testing"

	"zgo.at/kommentaar/docparse"
)

func TestExample(t *testing.T) {
	prog := docparse.NewProgram(false)
	prog.Config.Title = "Test Example"
	prog.Config.Version = "v1"
	prog.Config.Packages = []string{"../example/..."}
	prog.Config.Output = WriteJSONIndent

	w := bytes.NewBufferString("")
	err := docparse.FindComments(w, prog)
	if err != nil {
		t.Fatal(err)
	}

	if len(w.String()) < 500 {
		t.Errorf("short output?")
	}
}
