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
measurements in a one command tool. Runs the benchmarks multiple times
automatically. It is designed to be usable as part of github actions.

Example:

```
$ ba -a HEAD~1
go test -bench . -benchtime 100ms -count 5 -run ^$ -cpu 1 ./...
Checking out HEAD~1
go test -bench . -benchtime 100ms -count 5 -run ^$ -cpu 1 ./...
Checking out 02152d698f7d548cc86be35a7da3fc0aee93dec1
go test -bench . -benchtime 100ms -count 5 -run ^$ -cpu 1 ./...
Checking out HEAD~1
go test -bench . -benchtime 100ms -count 5 -run ^$ -cpu 1 ./...
Checking out 02152d698f7d548cc86be35a7da3fc0aee93dec1
name                  old time/op    new time/op    delta
HashCommand             69.1ns ± 1%    66.7ns ± 2%  -3.47%  (p=0.000 n=10+10)
CLParser                 280µs ± 1%     281µs ± 2%    ~     (p=0.739 n=10+10)
LoadManifest             432ms ± 5%     429ms ± 6%    ~     (p=0.529 n=10+10)
CanonicalizePathBits    85.7ns ± 1%    85.5ns ± 1%    ~     (p=0.118 n=10+10)
CanonicalizePath        83.6ns ± 1%    84.0ns ± 1%    ~     (p=0.239 n=10+10)

name                  old alloc/op   new alloc/op   delta
HashCommand              0.00B          0.00B         ~     (all equal)
CLParser                 164kB ± 0%     164kB ± 0%    ~     (all equal)
LoadManifest             298MB ± 0%     295MB ± 0%  -0.78%  (p=0.000 n=9+10)
CanonicalizePathBits     80.0B ± 0%     80.0B ± 0%    ~     (all equal)
CanonicalizePath         80.0B ± 0%     80.0B ± 0%    ~     (all equal)

name                  old allocs/op  new allocs/op  delta
HashCommand               0.00           0.00         ~     (all equal)
CLParser                 1.64k ± 0%     1.64k ± 0%    ~     (all equal)
LoadManifest             2.61M ± 0%     2.57M ± 0%  -1.71%  (p=0.000 n=9+10)
CanonicalizePathBits      1.00 ± 0%      1.00 ± 0%    ~     (all equal)
CanonicalizePath          1.00 ± 0%      1.00 ± 0%    ~     (all equal)
```

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
