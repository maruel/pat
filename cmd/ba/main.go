// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// ba bench against a base commit.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"golang.org/x/perf/benchstat"
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

// runBenchmarks runs benchmarks and return the go test -bench=. result for
// (old, new) where old is `against` and new is HEAD.
func runBenchmarks(ctx context.Context, against, pkg, b string, duration time.Duration, count, series int) (string, string, error) {
	// Make sure the tree is checked out and pristine, otherwise we could loose the checkout.
	diff, err := git("status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if diff != "" {
		return "", "", errors.New("the tree is modified, make sure to commit all your changes before running this script")
	}

	// Verify current and against are different commits.
	sha1Cur, err := git("rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	sha1Ag, err := git("rev-parse", against)
	if err != nil {
		return "", "", err
	}
	if sha1Cur == sha1Ag {
		return "", "", errors.New("specify -a to state against why commit to test, e.g. -a HEAD~1")
	}

	// Make sure we'll be able to check the commit back.
	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", "", err
	}
	if branch == "HEAD" {
		// We're in detached head. It's fine, just save the head.
		branch = sha1Cur[:16]
	}

	commitsHashes, err := git("log", "--format='%h'", sha1Cur+"..."+sha1Ag)
	if err != nil {
		return "", "", err
	}
	commits := strings.Count(commitsHashes, "\n") + 1

	// Run the benchmarks.
	// TODO(maruel): Make it smart, where it does series until the numbers
	// becomes stable, and actively ignores the higher values.
	// TODO(maruel): When a benchmark takes more than duration*count, reduce its count to 1.
	oldStats := ""
	newStats := ""
	needRevert := false
	fmt.Fprintf(os.Stderr, "%s...%s (%d commits), %s x %d times/batch, batch repeated %d times.\n", branch, against, commits, duration, count, series)
	for i := 0; i < series; i++ {
		if ctx.Err() != nil {
			// Don't error out, just quit.
			break
		}
		out := ""
		out, err = bench(ctx, pkg, b, duration, count)
		if err != nil {
			break
		}
		newStats += out

		fmt.Fprintf(os.Stderr, "git checkout %s\n", against)
		needRevert = true
		if out, err = git("checkout", "-q", against); err != nil {
			err = fmt.Errorf(out)
			break
		}
		out, err = bench(ctx, pkg, b, duration, count)
		if err != nil {
			break
		}
		oldStats += out
		fmt.Fprintf(os.Stderr, "git checkout %s\n", branch)
		if out, err = git("checkout", "-q", branch); err != nil {
			err = fmt.Errorf(out)
			break
		}
		needRevert = false
	}
	if needRevert {
		fmt.Fprintf(os.Stderr, "Checking out %s\n", branch)
		out := ""
		if out, err = git("checkout", "-q", branch); err != nil {
			err = fmt.Errorf(out)
		}
	}
	return oldStats, newStats, err
}

func printBenchstat(w io.Writer, o, n string) error {
	c := &benchstat.Collection{
		Alpha:      0.05,
		AddGeoMean: false,
		DeltaTest:  benchstat.UTest,
	}
	// benchstat assumes that old must be first!
	if err := c.AddFile("HEAD~1", strings.NewReader(o)); err != nil {
		return err
	}
	if err := c.AddFile("HEAD", strings.NewReader(n)); err != nil {
		return err
	}
	benchstat.FormatText(w, c.Tables())
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		cancel()
	}()
	oldStats, newStats, err := runBenchmarks(ctx, *against, *pkg, *b, *duration, *count, *series)

	buf := bytes.Buffer{}
	if err2 := printBenchstat(&buf, oldStats, newStats); err2 != nil {
		return err2
	}
	if _, err2 := os.Stdout.Write(buf.Bytes()); err2 != nil {
		return err2
	}
	return err
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "ba: %s\n", err)
		os.Exit(1)
	}
}
