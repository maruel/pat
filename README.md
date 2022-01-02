# pat

Performance Analysis Toolbox for Go programs.

## Usage

```
go install github.com/maruel/pat/cmd/...@latest

# Print all the slice boundary checks in file util.go by building ./cmd/nin:
boundcheck -pkg ./cmd/nin -file util.go

# Disassemble nin.CanonicalizePath() when building ./cmd/nin:
disfunc -f 'nin\.CanonicalizePath$' -pkg ./cmd/nin
```
