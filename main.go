package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

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
var goTestOutputMatcher = regexp.MustCompile(`\s+[\w.]+:\d+:\s(.+)`)

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
	scanner := bufio.NewScanner(os.Stdin)

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

		fmt.Printf("| %s | %s | %s | %s | %s | %.2f | %d |\n",
			gtl.Test, ell.Sample.Input, ell.Sample.Expected, ell.Sample.Output, ell.Result.Type, ell.Result.Score, ell.Duration.Milliseconds())

		score += ell.Result.Score
		duration += ell.Duration
		n++
	}

	// Print table footer with total score
	fmt.Println("| | | | | | | |")
	fmt.Printf("| **Total** | | | | | **%.2f** | **%d** |\n", float64(score)/float64(n), duration.Milliseconds())

	return nil
}
