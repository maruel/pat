// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnnotated(t *testing.T) {
	locs, err := getLocs(".", filepath.Join(t.TempDir(), "foo"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	buf := bytes.Buffer{}
	printAnnotated(&buf, locs)
	got := buf.String()
	if !strings.Contains(got, "; main.printAnnotated(SB)\n") || !strings.Contains(got, "; main.getLocs(SB)\n") {
		t.Fatal(got)
	}
}

func TestRaw(t *testing.T) {
	locs, err := getLocs(".", filepath.Join(t.TempDir(), "foo"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	buf := bytes.Buffer{}
	printRaw(&buf, locs)
	got := buf.String()
	if c := strings.Count(got, "main.go:"); c < 5 || c > 10 {
		t.Fatal(got)
	}
}

func TestTerse(t *testing.T) {
	locs, err := getLocs(".", filepath.Join(t.TempDir(), "foo"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	buf := bytes.Buffer{}
	printTerse(&buf, locs)
	got := buf.String()
	if c := strings.Count(got, "main.go:"); c != 1 {
		t.Fatal(got)
	}
}
