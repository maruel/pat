// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// ba bench against a base commit.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	// TODO(maruel): Figure this out.
	"golang.org/x/perf/benchstat"
)

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func runBench(ctx context.Context, pkg, bench string, benchtime time.Duration, count int) (string, error) {
	args := []string{
		"test",
		"-bench", bench,
		"-benchtime", benchtime.String(),
		"-count", strconv.Itoa(count),
		"-run", "^$",
		"-cpu", "1",
	}
	if pkg != "" {
		args = append(args, pkg)
	}
	fmt.Fprintf(os.Stderr, "go %s\n", strings.Join(args, " "))
	/* #nosec G204 */
	out, err := exec.CommandContext(ctx, "go", args...).CombinedOutput()
	return string(out), err
}

// isPristine makes sure the tree is checked out and pristine, otherwise we
// could loose the checkout.
func isPristine() error {
	diff, err := git("status", "--porcelain")
	if err != nil {
		return err
	}
	if diff != "" {
		return errors.New("the tree is modified, make sure to commit all your changes before running this script")
	}
	return nil
}

func getInfos(against string) (string, int, error) {
	// Verify current and against are different commits.
	sha1Cur, err := git("rev-parse", "HEAD")
	if err != nil {
		return "", 0, err
	}
	sha1Ag, err := git("rev-parse", against)
	if err != nil {
		return "", 0, err
	}
	if sha1Cur == sha1Ag {
		return "", 0, errors.New("specify -against to state against why commit to test, e.g. -against HEAD~1")
	}

	// Make sure we'll be able to check the commit back.
	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", 0, err
	}
	if branch == "HEAD" {
		// We're in detached head. It's fine, just save the head.
		branch = sha1Cur[:16]
	}

	commitsHashes, err := git("log", "--format='%h'", sha1Cur+"..."+sha1Ag)
	if err != nil {
		return "", 0, err
	}
	commits := strings.Count(commitsHashes, "\n") + 1
	return branch, commits, nil
}

func warmBench(ctx context.Context, branch, against, pkg, bench string, benchtime time.Duration) error {
	fmt.Fprintf(os.Stderr, "warming up\n")
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := runBench(ctx, pkg, bench, benchtime, 1); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "git checkout %s\n", against)
	out, err := git("checkout", "-q", against)
	if err == nil {
		_, err = runBench(ctx, pkg, bench, benchtime, 1)
	} else {
		err = errors.New(out)
	}
	fmt.Fprintf(os.Stderr, "git checkout %s\n", branch)
	if out2, err2 := git("checkout", "-q", branch); err2 != nil {
		return errors.New(out2)
	}
	return err
}

// runBenchmarks runs benchmarks and return the go test -bench=. result for
// (old, new) where old is `against` and new is HEAD.
func runBenchmarks(ctx context.Context, against, pkg, bench string, benchtime time.Duration, count, series int, nowarm bool) (string, string, error) {
	if err := isPristine(); err != nil {
		return "", "", err
	}
	branch, commits, err := getInfos(against)
	if err != nil {
		return "", "", err
	}

	// TODO(maruel): Make it smart, where it does series until the numbers
	// becomes stable, and actively ignores the higher values.
	// TODO(maruel): When a benchmark takes more than benchtime*count, reduce its
	// count to 1. We could do this by running -benchtime=1x -json.
	// This is particularly problematic with benchmarks lasting less than 100ns
	// per operation as they fail to be numerically stable and deviate by ~3%.
	if !nowarm {
		if err = warmBench(ctx, branch, against, pkg, bench, benchtime); err != nil {
			return "", "", err
		}
	}

	// Run the benchmarks.
	oldStats := ""
	newStats := ""
	needRevert := false
	fmt.Fprintf(os.Stderr, "%s...%s (%d commits), %s x %d times/batch, batch repeated %d times.\n", branch, against, commits, benchtime, count, series)
	for i := 0; i < series; i++ {
		if ctx.Err() != nil {
			// Don't error out, just quit.
			break
		}
		out := ""
		out, err = runBench(ctx, pkg, bench, benchtime, count)
		if err != nil {
			break
		}
		newStats += out

		fmt.Fprintf(os.Stderr, "git checkout %s\n", against)
		needRevert = true
		if out, err = git("checkout", "-q", against); err != nil {
			err = errors.New(out)
			break
		}
		out, err = runBench(ctx, pkg, bench, benchtime, count)
		if err != nil {
			break
		}
		oldStats += out
		fmt.Fprintf(os.Stderr, "git checkout %s\n", branch)
		if out, err = git("checkout", "-q", branch); err != nil {
			err = errors.New(out)
			break
		}
		needRevert = false
	}
	if needRevert {
		fmt.Fprintf(os.Stderr, "Checking out %s\n", branch)
		out := ""
		if out, err = git("checkout", "-q", branch); err != nil {
			err = errors.New(out)
		}
	}
	return oldStats, newStats, err
}

func genBenchTables(against, head, o, n string) ([]*benchstat.Table, error) {
	c := &benchstat.Collection{
		Alpha:     0.05,
		DeltaTest: benchstat.UTest,
	}
	// benchstat assumes that old must be first!
	if err := c.AddFile(against, strings.NewReader(o)); err != nil {
		return nil, err
	}
	if err := c.AddFile(head, strings.NewReader(n)); err != nil {
		return nil, err
	}
	return c.Tables(), nil
}

func printBenchstat(w io.Writer, tables []*benchstat.Table) error {
	benchstat.FormatText(w, tables)
	return nil
}

func jsonBenchstat(w io.Writer, tables []*benchstat.Table) error {
	out := make([]*jsonTable, 0, len(tables))
	for _, t := range tables {
		outt := &jsonTable{
			Metric:  t.Metric,
			Unit:    t.Rows[0].Metrics[0].Unit,
			Configs: t.Configs,
			Rows:    make([]*jsonRow, 0, len(t.Rows)),
		}
		for _, row := range t.Rows {
			r := &jsonRow{
				Benchmark: row.Benchmark,
				Metrics:   make([]*jsonMetrics, 0, len(row.Metrics)),
				PctDelta:  row.PctDelta,
				Delta:     row.Delta,
				Note:      row.Note,
				Change:    row.Change,
			}
			for _, m := range row.Metrics {
				r.Metrics = append(r.Metrics, &jsonMetrics{
					Values:  m.Values,
					RValues: m.RValues,
					Min:     m.Min,
					Mean:    m.Mean,
					Max:     m.Max,
				})
			}
			outt.Rows = append(outt.Rows, r)
		}
		out = append(out, outt)
	}
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(out)
}

type jsonTable struct {
	Metric  string
	Unit    string
	Configs []string
	Rows    []*jsonRow
}

type jsonRow struct {
	Benchmark string
	Metrics   []*jsonMetrics
	PctDelta  float64
	Delta     string
	Note      string
	Change    int
}

type jsonMetrics struct {
	Values  []float64 // measured values
	RValues []float64 // Values with outliers removed
	Min     float64   // min of RValues
	Mean    float64   // mean of RValues
	Max     float64   // max of RValues
}

func mainImpl() error {
	// Reduce runtime interference. 'ba' is meant to be relatively short running
	// and the amount of data processed is small so GC is unnecessary.
	runtime.LockOSThread()
	debug.SetGCPercent(0)
	pkg := flag.String("pkg", "./...", "package to bench")
	bench := flag.String("bench", ".", "benchmark to run, default to all")
	against := flag.String("against", "origin/main", "commitref to benchmark against")
	benchtime := flag.Duration("benchtime", 100*time.Millisecond, "duration of each benchmark")
	format := flag.String("format", "text", "format to print; either text or json")
	count := flag.Int("count", 2, "count to run per attempt")
	series := flag.Int("series", 3, "series to run the benchmark")
	// TODO(maruel): This does not seem to help.
	nowarm := flag.Bool("nowarm", true, "do not run an extra warmup series")
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
		return errors.New("unexpected argument")
	}
	switch *format {
	case "text", "json":
	default:
		return errors.New("unsupported -format")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		cancel()
	}()

	oldStats, newStats, err := runBenchmarks(ctx, *against, *pkg, *bench, *benchtime, *count, *series, *nowarm)
	t, err2 := genBenchTables(*against, "HEAD", oldStats, newStats)
	if err == nil {
		err = err2
	}
	if err != nil {
		return err
	}
	switch *format {
	case "text":
		err = printBenchstat(os.Stdout, t)
	case "json":
		err = jsonBenchstat(os.Stdout, t)
	default:
		err = errors.New("internal error")
	}
	return err
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "ba: %s\n", err)
		os.Exit(1)
	}
}
