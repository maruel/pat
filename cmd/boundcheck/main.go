// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// boundcheck prints out bound checks.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/mgutz/ansi"
)

type loc struct {
	file string
	line int
}

func printRaw(locs []loc, file string) {
	for _, l := range locs {
		if file != "" && l.file != file {
			continue
		}
		fmt.Printf("%s:%d\n", l.file, l.line)
	}
}

func printTerse(locs []loc, file string) {
	m := map[string][]int{}
	var names []string
	for _, l := range locs {
		if file != "" && l.file != file {
			continue
		}
		if _, ok := m[l.file]; !ok {
			names = append(names, l.file)
		}
		m[l.file] = append(m[l.file], l.line)
	}
	sort.Strings(names)
	for _, n := range names {
		out := ""
		for i, l := range m[n] {
			if i != 0 {
				out += ", "
			}
			out += strconv.Itoa(l)
		}
		fmt.Printf("%s: %s\n", n, out)
	}
}

func printAnnotated(w io.Writer, locs []loc, file string) {
	m := map[string][]int{}
	var names []string
	for _, l := range locs {
		if file != "" && l.file != file {
			continue
		}
		if _, ok := m[l.file]; !ok {
			names = append(names, l.file)
		}
		m[l.file] = append(m[l.file], l.line)
	}
	sort.Strings(names)

	for _, n := range names {
		d, err := ioutil.ReadFile(n)
		if err != nil {
			// Silently ignore files for now.
			continue
		}
		lines := strings.Split(string(d), "\n")
		fmt.Fprintf(w, "%s\n", n)
		for i, l := range m[n] {
			fmt.Fprintf(w, "% 5d %s\n", l-1, shorten(lines[l-2]))
			fmt.Fprintf(w, "% 5d %s\n", l, highlight(shorten(lines[l-1])))
			fmt.Fprintf(w, "% 5d %s\n", l+1, shorten(lines[l]))
			if i != 0 {
				fmt.Fprintf(w, "\n")
			}
		}
	}
}

func shorten(l string) string {
	return strings.ReplaceAll(l, "\t", "  ")
}

func highlight(l string) string {
	t := ""
	inQuote := false
	inDoubleQuote := false
	inBracket := 0
	for i := 0; i < len(l); i++ {
		switch c := l[i]; c {
		case '[':
			if !inQuote && !inDoubleQuote {
				inBracket++
				if inBracket == 1 {
					t += ansi.ColorCode("red+b")
				}
			}
			t += string(c)
		case ']':
			t += string(c)
			if !inQuote && !inDoubleQuote {
				inBracket--
				if inBracket == 0 {
					t += ansi.Reset
				}
			}
		case '\'':
			if !inDoubleQuote {
				inQuote = !inQuote
			}
			t += string(c)
		case '"':
			if !inQuote {
				inDoubleQuote = !inDoubleQuote
			}
			t += string(c)
		default:
			t += string(c)
		}
	}
	return t
}

func getLocs(pkg, bin, filter string) ([]loc, error) {
	if err := exec.Command("go", "build", "-o", bin, pkg).Run(); err != nil {
		return nil, err
	}

	args := []string{"tool", "objdump"}
	if filter != "" {
		args = append(args, "-s", filter+"\\.")
	}
	args = append(args, bin)
	disasmOut, err := exec.Command("go", args...).Output()
	if err != nil {
		return nil, err
	}

	var locs []loc
	for _, l := range strings.Split(string(disasmOut), "\n") {
		if strings.Contains(l, "CALL runtime.panicIndex") {
			l = strings.TrimSpace(l)
			i := strings.IndexByte(l, ':')
			j := strings.IndexByte(l, '\t')
			n, err := strconv.Atoi(l[i+1 : j])
			if err != nil {
				return nil, err
			}
			locs = append(locs, loc{l[:i], n})
		}
	}
	sort.Slice(locs, func(i, j int) bool {
		x := locs[i]
		y := locs[j]
		if x.file != y.file {
			return x.file < y.file
		}
		return x.line < y.line
	})
	return locs, nil
}

func mainImpl() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	pkg := flag.String("pkg", ".", "package to build, preferably an executable")
	bin := flag.String("bin", filepath.Base(wd), "binary to generate")
	filter := flag.String("f", "", "package to filter symbols on")
	raw := flag.Bool("raw", false, "raw output")
	terse := flag.Bool("terse", false, "terse output")
	file := flag.String("file", "", "filter on one file")
	flag.Usage = func() {
		fmt.Printf("usage: boundcheck <flags>\n")
		fmt.Printf("\n")
		fmt.Printf("boundcheck prints out all the lines that the compiler inserted\n")
		fmt.Printf("a slice bound check in.\n")
		fmt.Printf("\n")
		fmt.Printf("example:\n")
		fmt.Printf("  boundcheck -f nin -pkg ./cmd/nin -file util.go\n")
		fmt.Printf("\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	locs, err := getLocs(*pkg, *bin, *filter)
	if err != nil {
		return err
	}

	if *raw {
		printRaw(locs, *file)
		return nil
	}

	if *terse {
		printTerse(locs, *file)
		return nil
	}

	var w io.Writer = os.Stdout
	if isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("TERM") != "dumb" {
		w = colorable.NewColorableStdout()
	}
	printAnnotated(w, locs, *file)
	return nil
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "boundcheck: %s\n", err)
		os.Exit(1)
	}
}
