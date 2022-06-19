# pat

Performance Analysis Toolbox for Go programs.

[![PkgGoDev](https://pkg.go.dev/badge/github.com/maruel/pat)](https://pkg.go.dev/github.com/maruel/pat)
[![codecov](https://codecov.io/gh/maruel/pat/branch/main/graph/badge.svg?token=UNE311HJM8)](https://codecov.io/gh/maruel/pat)

## Usage

Get with:

```
go install github.com/maruel/pat/cmd/...@latest
```

## ba

ba benches against a base git commit, providing more stable benchmark
measurements in a one command tool.

## disfunc

Disassemble a function at the command line with source annotation.

Example: disassemble function nin.CanonicalizePath() when building ./cmd/nin:

```
disfunc -f 'nin\.CanonicalizePath$' -pkg ./cmd/nin | less -R
```

Colors:

- Green:  calls/returns
- Red:    panic() due to bound checking and traps
- Blue:   jumps (both conditional and unconditional)
- Violet: padding and noops
- Yellow: source code; bound check highlighted red

![screenshot](https://github.com/maruel/pat/wiki/disfunc.png)

disfunc uses `go tool objdump` output.

## boundcheck

Lists all the bound checks in a source file or package. Useful to do a quick
audit:

```
boundcheck -pkg ./cmd/nin | less -R
```

![screenshot](https://github.com/maruel/pat/wiki/boundcheck.png)

boundcheck uses `go tool objdump` output. An alternative way is to use `go build
-gcflags="-d=ssa/check_bce/debug=1"`
