// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"testing"
	"time"
)

func TestSpinCPU(t *testing.T) {
	buf := bytes.Buffer{}
	x := spinCPU(&buf, time.Microsecond)
	if x != 5161808367612500732 {
		t.Fatal(x)
	}
	if s := buf.String(); s != "Spinning for 1µs.\n" {
		t.Fatal(s)
	}
}
