package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"maragu.dev/evals/internal/sql"
)

type evalLogLine struct {
	Name     string
	Sample   Sample
	Results  []Result
	Duration time.Duration
}

type Sample struct {
	Expected string
	Input    string
	Output   string
}

type Result struct {
	Score Score
	Type  string
}

type Score float64

func main() {
	if err := start(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func start() error {
	ctx := context.Background()

	input := flag.String("i", "evals.jsonl", "input file path")
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

	var totalScore Score
	var n int

	var outputLines []string

	for scanner.Scan() {
		var ell evalLogLine
		b := scanner.Bytes()
		if err := json.Unmarshal(b, &ell); err != nil {
			return fmt.Errorf("error unmarshalling line: %w", err)
		}

		for _, result := range ell.Results {
			var previousScore Score
			var newScore bool
			if err := h.Get(ctx, &previousScore, `select score from evals where name = ? and type = ? order by experiment desc limit 1`, ell.Name, result.Type); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("error getting previous score from database: %w", err)
				}
				newScore = true
			}

			if n == 0 {
				fmt.Println("| Name | Type | Score | Duration |")
				fmt.Println("| --- | --- | --- | --: |")
			}

			var scoreChange string
			switch {
			case newScore:
				scoreChange = " (new)"
			case result.Score-previousScore >= 0.005:
				scoreChange = fmt.Sprintf(" (+%.2f)", result.Score-previousScore)
			case previousScore-result.Score >= 0.005:
				scoreChange = fmt.Sprintf(" (-%.2f)", previousScore-result.Score)
			}

			outputLine := fmt.Sprintf("| %s | %s | %.2f %v | %v |\n", ell.Name, result.Type, result.Score, scoreChange, roundDuration(ell.Duration))
			outputLines = append(outputLines, outputLine)

			err := h.Exec(ctx, `insert into evals (experiment, name, input, expected, output, type, score, duration) values (?, ?, ?, ?, ?, ?, ?, ?)`,
				*experiment, ell.Name, ell.Sample.Input, ell.Sample.Expected, ell.Sample.Output, result.Type, result.Score, ell.Duration)
			if err != nil {
				return fmt.Errorf("error inserting eval into database: %w", err)
			}

			totalScore += result.Score
			n++
		}
	}

	// Sort output lines by name, type
	slices.Sort(outputLines)
	for _, line := range outputLines {
		fmt.Print(line)
	}

	if n > 0 {
		// Print table footer with total score
		fmt.Printf("| **Total** | | **%.2f** | |\n", float64(totalScore)/float64(n))
	}

	return nil
}

func roundDuration(v time.Duration) time.Duration {
	if v < time.Second {
		return v.Round(time.Millisecond)
	}
	return v.Round(100 * time.Millisecond)
}
