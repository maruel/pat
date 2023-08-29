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

`ba` benches against a base git commit, providing more stable benchmark
measurements. ba leverages
[golang.org/x/perf/benchstat](https://golang.org/x/perf/benchstat) for benchmark
performance difference calculation.

It runs the benchmarks multiple times in alternation to reduce the variance
while taking as little time as possible. It is designed to be usable as part of
github actions.

Example:

```
$ ba -against HEAD~1
warming up
go test -bench . -benchtime 100ms -count 1 -run ^$ -cpu 1 ./...
git checkout HEAD~1
go test -bench . -benchtime 100ms -count 1 -run ^$ -cpu 1 ./...
git checkout 02152d698f7d548c
02152d698f7d548c...HEAD~1 (1 commits), 100ms x 2 times/batch, batch repeated 3 times.
go test -bench . -benchtime 100ms -count 2 -run ^$ -cpu 1 ./...
git checkout HEAD~1
go test -bench . -benchtime 100ms -count 2 -run ^$ -cpu 1 ./...
git checkout 02152d698f7d548c
go test -bench . -benchtime 100ms -count 2 -run ^$ -cpu 1 ./...
git checkout HEAD~1
go test -bench . -benchtime 100ms -count 2 -run ^$ -cpu 1 ./...
git checkout 02152d698f7d548c
go test -bench . -benchtime 100ms -count 2 -run ^$ -cpu 1 ./...
git checkout HEAD~1
go test -bench . -benchtime 100ms -count 2 -run ^$ -cpu 1 ./...
git checkout 02152d698f7d548c
name                  old time/op    new time/op    delta
HashCommand             69.0ns ± 2%    67.7ns ± 2%  -1.91%  (p=0.041 n=6+6)
CLParser                 281µs ± 1%     281µs ± 1%    ~     (p=0.699 n=6+6)
LoadManifest             437ms ± 7%     430ms ± 3%    ~     (p=0.937 n=6+6)
CanonicalizePathBits    85.9ns ± 1%    86.2ns ± 0%    ~     (p=1.000 n=6+6)
CanonicalizePath        83.9ns ± 1%    84.6ns ± 0%    ~     (p=0.058 n=6+6)

name                  old alloc/op   new alloc/op   delta
HashCommand              0.00B          0.00B         ~     (all equal)
CLParser                 164kB ± 0%     164kB ± 0%    ~     (all equal)
LoadManifest             298MB ± 0%     295MB ± 0%  -0.78%  (p=0.002 n=6+6)
CanonicalizePathBits     80.0B ± 0%     80.0B ± 0%    ~     (all equal)
CanonicalizePath         80.0B ± 0%     80.0B ± 0%    ~     (all equal)

name                  old allocs/op  new allocs/op  delta
HashCommand               0.00           0.00         ~     (all equal)
CLParser                 1.64k ± 0%     1.64k ± 0%    ~     (all equal)
LoadManifest             2.61M ± 0%     2.57M ± 0%  -1.71%  (p=0.002 n=6+6)
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
