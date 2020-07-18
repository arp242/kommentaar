package srvhttp

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/teamwork/test/diff"
)

func TestServe(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	args := Args{
		Packages: []string{"../example/..."},
	}

	rr := httptest.NewRecorder()
	JSON(args)(rr, r)
	if len(rr.Body.String()) < 500 {
		t.Error("body too short for JSON?")
	}

	rr = httptest.NewRecorder()
	JSON(args)(rr, r)
	if len(rr.Body.String()) < 500 {
		t.Error("body too short for JSON?")
	}

	rr = httptest.NewRecorder()
	HTML(args)(rr, r)
	if len(rr.Body.String()) < 500 {
		t.Error("body too short for HTML?")
	}
}

func TestFromFile(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	args := Args{
		Packages: []string{"../example/..."},
		NoScan:   true,
		JSONFile: "../testdata/openapi2/src/blank-line/want.json",
	}

	want, err := ioutil.ReadFile("../testdata/openapi2/src/blank-line/want.json")
	if err != nil {
		t.Fatalf("could not read file: %v", err)
	}

	rr := httptest.NewRecorder()
	JSON(args)(rr, r)
	d := diff.TextDiff(string(want), rr.Body.String())
	if d != "" {
		t.Fatalf("wrong output\n%v", d)
	}
}
