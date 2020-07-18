package kconfig

import (
	"testing"

	"github.com/zgoat/kommentaar/docparse"
	"zgo.at/ztest"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"example", ztest.Read(t, "../config.example")},
		{"default-response", []byte(ztest.NormalizeIndent(`
			default-response 400: github.com/zgoat/kommentaar/docparse.Param
			default-response 404 (application/json): net/mail.Address
		`))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, clean := ztest.TempFile(t, string(tt.in))
			defer clean()

			prog := docparse.NewProgram(false)

			err := Load(prog, f)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
		})
	}
}
