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
	s, err := getDisasm(".", filepath.Join(t.TempDir(), "foo"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	buf := bytes.Buffer{}
	printAnnotated(&buf, s)
	got := buf.String()
	if !strings.Contains(got, "main.printAnnotated.func1(SB)") {
		t.Fatal(got)
	}
}
