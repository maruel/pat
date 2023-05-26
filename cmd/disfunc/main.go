// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// disfunc disassemble a function.
package main

import (
	"flag"
	"fmt"
	"io"
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

type disasmLine struct {
	index     int
	file      string // util.go
	fileSrc   string // util.go:123
	srcLine   int    // 123
	binOffset int    // Binary offset from the start of the executable
	symOffset int    // Binary offset from the start of the symbol
	asm       string // raw bytes
	decoded   string // full decoded instruction
	instr     string // only the instruction
	arg       string // only arguments
	alias     string // processed arguments, when applicable
}

type disasmSym struct {
	file      string
	symbol    string
	binOffset int // Binary offset from the start of the executable
	content   []*disasmLine
}

func getDisasm(pkg, bin, filter, file string) ([]*disasmSym, error) {
	if err := exec.Command("go", "build", "-o", bin, pkg).Run(); err != nil {
		return nil, err
	}

	args := []string{"tool", "objdump"}
	if filter != "" {
		args = append(args, "-s", filter)
	}
	args = append(args, bin)
	disasmOut, err := exec.Command("go", args...).Output()
	if err != nil {
		return nil, err
	}

	var out []*disasmSym
	const textPrefix = "TEXT "
	m := map[int]*disasmLine{}
	index := 0
	for _, l := range strings.Split(string(disasmOut), "\n") {
		if l == "" {
			index = 0
			continue
		}
		if strings.HasPrefix(l, textPrefix) {
			// TEXT github.com/maruel/nin.CanonicalizePath(SB) /home/maruel/src/nin/util.go
			f := strings.SplitN(l[len(textPrefix):], " ", 2)
			if len(f) != 2 {
				return nil, fmt.Errorf("error decoding %q", l)
			}
			d := &disasmSym{
				file:   f[1],
				symbol: f[0],
			}
			out = append(out, d)
			index = 0
			continue
		}
		if !strings.HasPrefix(l, "  ") || len(out) == 0 {
			return nil, fmt.Errorf("error decoding %q", l)
		}
		d := out[len(out)-1]
		// util.go:65            0x505dc0                4c8da42420feffff        LEAQ 0xfffffe20(SP), R12
		l = l[2:]
		i := strings.IndexByte(l, ':')
		j := strings.IndexByte(l, '\t')
		f := l[:i]
		fileSrc := l[:j]
		srcLine, err := strconv.Atoi(l[i+1 : j])
		if err != nil {
			return nil, err
		}
		l = strings.TrimSpace(l[j:])
		j = strings.IndexByte(l, '\t')
		binOffset, err := strconv.ParseInt(l[:j], 0, 0)
		if err != nil {
			return nil, err
		}
		l = strings.TrimSpace(l[j:])
		j = strings.IndexByte(l, '\t')
		asm := l[:j]
		decoded := strings.TrimSpace(l[j:])
		instr := decoded
		arg := ""
		if j = strings.IndexByte(decoded, ' '); j != -1 {
			instr = decoded[:j]
			arg = decoded[j+1:]
		}
		if len(d.content) == 0 {
			d.binOffset = int(binOffset)
		}
		a := &disasmLine{
			index:     index,
			file:      f,
			fileSrc:   fileSrc,
			srcLine:   srcLine,
			binOffset: int(binOffset),
			symOffset: int(binOffset) - d.binOffset,
			asm:       asm,
			decoded:   decoded,
			instr:     instr,
			arg:       arg,
		}
		d.content = append(d.content, a)
		m[int(binOffset)] = a
		index++
	}

	// After parsing everything, resolve the address of the jumps. Do this before
	// filtering just in case.
	for _, s := range out {
		for _, c := range s.content {
			// For any Jxx instruction, try to resolve the destination.
			if c.instr[0] == 'J' {
				if b, err := strconv.ParseInt(c.arg, 0, 0); err == nil {
					if dst := m[int(b)]; dst != nil {
						c.alias = fmt.Sprintf("%s (%d)", dst.fileSrc, dst.index)
					}
				}
			}
		}
	}

	if file != "" {
		// Trim out files after the fact. Do it inline if it is observed to be
		// performance critical.
		for i := 0; i < len(out); i++ {
			if filepath.Base(out[i].file) != file {
				copy(out[i:], out[i+1:])
				i--
			}
		}
	}
	return out, nil
}

func printAnnotated(w io.Writer, d []*disasmSym) {
	// Order blocks per file then per symbols.
	sort.Slice(d, func(i, j int) bool {
		x := d[i]
		y := d[j]
		if x.file != y.file {
			return x.file < y.file
		}
		return x.symbol < y.symbol
	})

	for _, s := range d {
		d, err := os.ReadFile(s.file)
		if err != nil {
			fmt.Fprintf(w, "couldn't read %q, skipping\n", s.file)
			continue
		}
		lines := strings.Split(string(d), "\n")
		fmt.Fprintf(w, "%s%s%s\n", ansi.LightYellow, s.symbol, ansi.Reset)

		// Reorder by line numbers to make it more easy to understand.
		sort.Slice(s.content, func(i, j int) bool {
			if s.content[i].srcLine != s.content[j].srcLine {
				return s.content[i].srcLine < s.content[j].srcLine
			}
			return s.content[i].index < s.content[j].index
		})

		lastLine := 0
		for i, c := range s.content {
			if c.srcLine != lastLine {
				// Print the source line. But first check if there's any panic before
				// the next block to highlight the line.
				lastLine = c.srcLine
				found := false
				for _, c2 := range s.content[i:] {
					if c2.srcLine != lastLine {
						break
					}
					if c2.instr == "CALL" && strings.HasPrefix(c2.arg, "runtime.panicIndex") {
						found = true
						break
					}
				}
				l := ""
				if c.srcLine >= 0 && c.srcLine < len(lines) {
					l = shorten(lines[c.srcLine-1])
					if found {
						l = highlightBracket(l)
					}
				}
				fmt.Fprintf(w, "%d  %s%s%s\n", c.srcLine, ansi.ColorCode("yellow+h+b"), l, ansi.Reset)
			}

			color := ""
			if c.instr == "CALL" || c.instr == "RET" {
				if strings.HasPrefix(c.arg, "runtime.panicIndex") {
					color = ansi.ColorCode("red+b")
				} else {
					color = ansi.LightGreen
				}
			} else if strings.HasPrefix(c.instr, "J") {
				color = ansi.LightBlue
			} else if c.instr == "UD2" {
				color = ansi.LightRed
			} else if c.instr == "INT" || strings.HasPrefix(c.instr, "NOP") {
				// Technically it should be INT 3
				color = ansi.LightMagenta
			}
			if arg := c.arg; arg != "" {
				if c.alias != "" {
					arg = c.alias
				}
				fmt.Fprintf(w, " %4d %s%-5s %s%s\n", c.index, color, c.instr, arg, ansi.Reset)
			} else {
				fmt.Fprintf(w, " %4d %s%s%s\n", c.index, color, c.instr, ansi.Reset)
			}

			// It's very ISA specific, only tested on x64 for now.
			// Inserts an empty line after unconditional control-flow modifying instructions (JMP, RET, UD2)
			if strings.HasPrefix(c.decoded, "JMP ") || strings.HasPrefix(c.decoded, "RET ") || strings.HasPrefix(c.decoded, "UD2 ") {
				fmt.Fprint(w, "\n")
			}
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

func mainImpl() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	pkg := flag.String("pkg", ".", "package to build, preferably an executable")
	bin := flag.String("bin", filepath.Base(wd), "binary to generate")
	filter := flag.String("f", "", "function to print out")
	//raw := flag.Bool("raw", false, "raw output")
	//terse := flag.Bool("terse", false, "terse output")
	file := flag.String("file", "", "filter on one file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: disfunc <flags>\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "disfunc prints out an annotated function.\n")
		fmt.Fprintf(os.Stderr, "It is recommended to use one of -f or -file.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Colors:\n")
		fmt.Fprintf(os.Stderr, "- Green:  calls/returns\n")
		fmt.Fprintf(os.Stderr, "- Red:    panic() due to bound checking and traps\n")
		fmt.Fprintf(os.Stderr, "- Blue:   jumps (both conditional and unconditional)\n")
		fmt.Fprintf(os.Stderr, "- Violet: padding and noops\n")
		fmt.Fprintf(os.Stderr, "- Yellow: source code; bound check highlighted red\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "example:\n")
		fmt.Fprintf(os.Stderr, "  disfunc -f 'nin\\.CanonicalizePath$' -pkg ./cmd/nin | less -R\n")
		fmt.Fprintf(os.Stderr, "\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	s, err := getDisasm(*pkg, *bin, *filter, *file)
	if err != nil {
		return err
	}

	var w io.Writer = os.Stdout
	if isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("TERM") != "dumb" {
		w = colorable.NewColorableStdout()
	}
	printAnnotated(w, s)
	return nil
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "disfunc: %s\n", err)
		os.Exit(1)
	}
}
