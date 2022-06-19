// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// ba bench against a base commit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func bench(ctx context.Context, pkg, b string, duration time.Duration, count int) (string, error) {
	args := []string{
		"test",
		"-bench",
		b,
		"-benchtime",
		duration.String(),
		"-count",
		strconv.Itoa(count),
		"-run",
		"^$",
		"-cpu",
		"1",
	}
	if pkg != "" {
		args = append(args, pkg)
	}
	fmt.Fprintf(os.Stderr, "go %s\n", strings.Join(args, " "))
	out, err := exec.CommandContext(ctx, "go", args...).CombinedOutput()
	return string(out), err
}

func run(c string, args ...string) error {
	cmd := exec.Command(c, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func writeTemp(c string) (string, error) {
	f, err := ioutil.TempFile("", "ba")
	if err != nil {
		return "", err
	}
	if _, err = f.WriteString(c); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func runBenchmarks(ctx context.Context, against, pkg, b string, duration time.Duration, count, series int) (string, string, error) {
	// Make sure we'll be able to check the commit back.
	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", "", err
	}
	if branch == "HEAD" {
		return "", "", errors.New("checkout a branch first")
	}
	// Make sure the tree is checked out and pristine, otherwise we could loose the checkout.
	diff, err := git("status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if diff != "" {
		return "", "", errors.New("the tree is modified, make sure to commit all your changes before running this script")
	}

	// Run the benchmarks.
	// TODO(maruel): Make it smart, where it does series until the numbers
	// becomes stable, and actively ignores the higher values.
	// TODO(maruel): When a benchmark takes more than duration*count, reduce its count to 1.
	oldStats := ""
	newStats := ""
	for i := 0; i < series; i++ {
		if ctx.Err() != nil {
			break
		}
		out, err := bench(ctx, pkg, b, duration, count)
		if err != nil {
			return "", "", err
		}
		newStats += out

		fmt.Fprintf(os.Stderr, "Checking out %s\n", against)
		if out, err = git("checkout", "-q", against); err != nil {
			return "", "", fmt.Errorf(out)
		}
		out, err = bench(ctx, pkg, b, duration, count)
		if err != nil {
			return "", "", err
		}
		oldStats += out
		fmt.Fprintf(os.Stderr, "Checking out %s\n", branch)
		if out, err = git("checkout", "-q", branch); err != nil {
			return "", "", fmt.Errorf(out)
		}
	}
	return oldStats, newStats, nil
}

func runBenchstat(o, n string) error {
	// Make sure benchstat is available before starting, if not install it.
	if out, _ := exec.Command("benchstat", "-help").CombinedOutput(); !strings.HasPrefix(string(out), "usage: benchstat [options] old.txt [new.txt] [more.txt ...]") {
		if err := run("go", "install", "golang.org/x/perf/cmd/benchstat@latest"); err != nil {
			return err
		}
	}
	of, err := writeTemp(o)
	if err != nil {
		return err
	}
	defer os.Remove(of)
	nf, err := writeTemp(n)
	if err != nil {
		return err
	}
	defer os.Remove(nf)
	// TODO(maruel): Take the output and color it.
	if err = run("benchstat", of, nf); err != nil {
		return err
	}
	return nil
}

func mainImpl() error {
	pkg := flag.String("pkg", "./...", "package to bench")
	b := flag.String("b", ".", "benchmark to run, default to all")
	count := flag.Int("c", 5, "count to run per attempt")
	against := flag.String("a", "origin/main", "commitref to benchmark against")
	duration := flag.Duration("d", 100*time.Millisecond, "duration of each benchmark")
	series := flag.Int("s", 2, "series to run the benchmark")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: ba <flags>\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "ba (benches against) run benchmarks on two different commits and\n")
		fmt.Fprintf(os.Stderr, "prints out the result with benchstat.\n")
		fmt.Fprintf(os.Stderr, "\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected argument.\n")
		os.Exit(1)
	}
	ctx := context.Background()
	oldStats, newStats, err := runBenchmarks(ctx, *against, *pkg, *b, *duration, *count, *series)
	if err != nil {
		return err
	}

	return runBenchstat(oldStats, newStats)
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "ba: %s\n", err)
		os.Exit(1)
	}
}
