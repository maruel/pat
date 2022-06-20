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
	sym  string
	file string
	line int
}

func printRaw(w io.Writer, locs []loc) {
	for _, l := range locs {
		fmt.Fprintf(w, "%s:%d\n", l.file, l.line)
	}
}

func printTerse(w io.Writer, locs []loc) {
	m := map[string][]int{}
	var names []string
	for _, l := range locs {
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
		fmt.Fprintf(w, "%s: %s\n", n, out)
	}
}

func printAnnotated(w io.Writer, locs []loc) {
	m := map[string][]loc{}
	var names []string
	for _, l := range locs {
		if _, ok := m[l.file]; !ok {
			names = append(names, l.file)
		}
		m[l.file] = append(m[l.file], l)
	}
	sort.Strings(names)

	for _, n := range names {
		/* #nosec G304 */
		d, err := ioutil.ReadFile(n)
		if err != nil {
			// Silently ignore files for now.
			continue
		}
		sym := ""
		lines := strings.Split(string(d), "\n")
		fmt.Fprintf(w, "%s\n", n)
		for _, l := range m[n] {
			id := l.line
			if l.sym != sym {
				sym = l.sym
				fmt.Fprintf(w, "; %s\n", sym)
			}
			fmt.Fprintf(w, "% 5d %s\n", id-1, shorten(lines[id-2]))
			fmt.Fprintf(w, "% 5d %s\n", id, highlightBracket(shorten(lines[id-1])))
			fmt.Fprintf(w, "% 5d %s\n", id+1, shorten(lines[id]))
			fmt.Fprintf(w, "\n")
		}
	}
}

func shorten(l string) string {
	return strings.ReplaceAll(l, "\t", "  ")
}

func highlightBracket(l string) string {
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

func getLocs(pkg, bin, filter, file string) ([]loc, error) {
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
	sym := ""
	const textPrefix = "TEXT "
	for _, l := range strings.Split(string(disasmOut), "\n") {
		if strings.HasPrefix(l, textPrefix) {
			f := strings.SplitN(l[len(textPrefix):], " ", 2)
			sym = f[0]
		}
		if strings.Contains(l, "CALL runtime.panicIndex") {
			l = strings.TrimSpace(l)
			i := strings.IndexByte(l, ':')
			j := strings.IndexByte(l, '\t')
			n, err := strconv.Atoi(l[i+1 : j])
			if err != nil {
				return nil, err
			}
			locs = append(locs, loc{sym, l[:i], n})
		}
	}
	if file != "" {
		for i := 0; i < len(locs); i++ {
			if locs[i].file != file {
				copy(locs[i:], locs[i+1:])
				locs = locs[:len(locs)-1]
				i--
			}
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
		fmt.Fprintf(os.Stderr, "usage: boundcheck <flags>\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "boundcheck prints out all the lines that the compiler inserted\n")
		fmt.Fprintf(os.Stderr, "a slice bound check in.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "example:\n")
		fmt.Fprintf(os.Stderr, "  boundcheck -f nin -pkg ./cmd/nin -file util.go\n")
		fmt.Fprintf(os.Stderr, "\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	locs, err := getLocs(*pkg, *bin, *filter, *file)
	if err != nil {
		return err
	}

	if *raw {
		printRaw(os.Stdout, locs)
		return nil
	}

	if *terse {
		printTerse(os.Stdout, locs)
		return nil
	}

	var w io.Writer = os.Stdout
	if isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("TERM") != "dumb" {
		w = colorable.NewColorableStdout()
	}
	printAnnotated(w, locs)
	return nil
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "boundcheck: %s\n", err)
		os.Exit(1)
	}
}
