package kconfig

import (
	"testing"

	"zgo.at/kommentaar/docparse"
	"zgo.at/zstd/ztest"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"example", ztest.Read(t, "../config.example")},
		{"default-response", []byte(ztest.NormalizeIndent(`
			default-response 400: zgo.at/kommentaar/docparse.Param
			default-response 404 (application/json): net/mail.Address
		`))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := ztest.TempFile(t, "", string(tt.in))

			prog := docparse.NewProgram(false)

			err := Load(prog, f)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
		})
	}
}
