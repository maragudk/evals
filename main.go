package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"maragu.dev/evals/internal/sql"
	"maragu.dev/llm/eval"
)

type goTestLine struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Output  string
}

// goTestOutputMatcher matches JSON output from the Go test line.
// See https://regex101.com/r/j5iQuq/latest
var goTestOutputMatcher = regexp.MustCompile(`\s+[\w.]+:\d+:\s({.+)`)

type evalLogLine struct {
	Sample   eval.Sample
	Result   eval.Result
	Duration time.Duration
}

func main() {
	if err := start(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func start() error {
	ctx := context.Background()

	input := flag.String("i", "-", "input file path, defaults to STDIN")
	experiment := flag.String("e", "", "experiment name")
	db := flag.String("db", "evals.db", "database file path, created if not exists")
	flag.Parse()

	var inputReader io.Reader = os.Stdin
	if input != nil && *input != "-" {
		file, err := os.Open(*input)
		if err != nil {
			return fmt.Errorf("error opening input file: %w", err)
		}
		defer func() {
			_ = file.Close()
		}()
		inputReader = file
	}

	if experiment == nil || *experiment == "" {
		// set the experiment name to the current time
		*experiment = time.Now().Format(time.RFC3339)
	}

	h := sql.NewHelper(sql.NewHelperOptions{Path: *db})
	if err := h.Connect(); err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	if err := h.MigrateUp(ctx); err != nil {
		return fmt.Errorf("error migrating database: %w", err)
	}

	scanner := bufio.NewScanner(inputReader)

	var tableHeaderIsOutput bool

	var score eval.Score
	var duration time.Duration
	var n int

	for scanner.Scan() {
		line := scanner.Bytes()
		var gtl goTestLine
		if err := json.Unmarshal(line, &gtl); err != nil {
			return fmt.Errorf("error unmarshalling line: %w", err)
		}

		if gtl.Action != "output" || !strings.HasPrefix(gtl.Test, "TestEval") {
			continue
		}

		if !goTestOutputMatcher.MatchString(gtl.Output) {
			continue
		}

		matches := goTestOutputMatcher.FindStringSubmatch(gtl.Output)

		var ell evalLogLine
		if err := json.Unmarshal([]byte(matches[1]), &ell); err != nil {
			return fmt.Errorf("error unmarshalling eval log line: %w", err)
		}

		if !tableHeaderIsOutput {
			fmt.Println("| Name | Input | Expected | Output | Type | Score | Duration |")
			fmt.Println("| --- | --- | --- | --- | --- | --: | --: |")
			tableHeaderIsOutput = true
		}

		fmt.Printf("| %s | %s | %s | %s | %s | %.2f | %v |\n",
			gtl.Test, ell.Sample.Input, ell.Sample.Expected, ell.Sample.Output, ell.Result.Type, ell.Result.Score, ell.Duration)

		err := h.Exec(ctx, `insert into evals (experiment, name, input, expected, output, type, score, duration) values (?, ?, ?, ?, ?, ?, ?, ?)`,
			*experiment, gtl.Test, ell.Sample.Input, ell.Sample.Expected, ell.Sample.Output, ell.Result.Type, ell.Result.Score, ell.Duration)
		if err != nil {
			return fmt.Errorf("error inserting eval into database: %w", err)
		}

		score += ell.Result.Score
		duration += ell.Duration
		n++
	}

	if tableHeaderIsOutput {
		// Print table footer with total score
		fmt.Println("| | | | | | | |")
		fmt.Printf("| **Total** | | | | | **%.2f** | **%v** |\n", float64(score)/float64(n), duration)
	}

	return nil
}
