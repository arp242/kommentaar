// Package srvhttp contains HTTP handlers for serving Kommentaar documentation.
package srvhttp

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"zgo.at/kommentaar/docparse"
	"zgo.at/kommentaar/html"
	"zgo.at/kommentaar/kconfig"
	"zgo.at/kommentaar/openapi2"
)

// Args for the HTTP handlers.
type Args struct {
	Packages []string // Packages to scan.
	Config   string   // Kommentaar config file.
	NoScan   bool     // Don't scan the paths, but instead load and output one of the *File.
	JSONFile string
	HTMLFile string
}

// JSON outputs as OpenAPI2 JSON.
//
// Set the "indented" query parameter to get formatted JSON.
func JSON(args Args) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var f func(io.Writer, *docparse.Program) error
		if r.URL.Query().Get("indented") != "" {
			f = openapi2.WriteJSONIndent
		} else {
			f = openapi2.WriteJSON
		}

		out, err := run(args, f, args.JSONFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, wErr := fmt.Fprintf(w, "Error: %v", err)
			if wErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "could not write response: %v", wErr)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, wErr := fmt.Fprint(w, out)
		if wErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "could not write response: %v", wErr)
		}
	}
}

// HTML outputs as HTML documentation.
func HTML(args Args) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := run(args, html.WriteHTML, args.HTMLFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, wErr := fmt.Fprintf(w, "Error: %v", err)
			if wErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "could not write response: %v", wErr)
			}
			return
		}

		w.Header().Set("Content-Type", "text/html")
		_, wErr := fmt.Fprint(w, out)
		if wErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "could not write response: %v", wErr)
		}
	}
}

func run(
	args Args,
	out func(io.Writer, *docparse.Program) error,
	file string,
) (string, error) {

	if args.NoScan {
		o, err := ioutil.ReadFile(file)
		return string(o), err
	}

	prog := docparse.NewProgram(false)
	if args.Config != "" {
		err := kconfig.Load(prog, args.Config)
		if err != nil {
			return "", err
		}
	}

	if len(args.Packages) > 0 {
		prog.Config.Packages = args.Packages
	}
	prog.Config.Output = out

	buf := bytes.NewBuffer(nil)
	err := docparse.FindComments(buf, prog)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
