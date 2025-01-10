package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"maragu.dev/llm/eval"

	"maragu.dev/evals/internal/sql"
	"maragu.dev/evals/internal/text"
)

type goTestLine struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Output  string
}

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

	parser := &text.Parser{}

	for scanner.Scan() {
		ell, ok := parser.Parse(scanner.Text())
		if !ok {
			continue
		}

		var previousScore eval.Score
		var newScore bool
		if err := h.Get(ctx, &previousScore, `select score from evals where name = ? order by experiment desc limit 1`, ell.Name); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("error getting previous score from database: %w", err)
			}
			newScore = true
		}

		if !tableHeaderIsOutput {
			fmt.Println("| Name | Input | Expected | Output | Type | Score | Duration |")
			fmt.Println("| --- | --- | --- | --- | --- | --: | --: |")
			tableHeaderIsOutput = true
		}

		// Set a max length for the strings in the markdown table
		input := ell.Sample.Input
		expected := ell.Sample.Expected
		output := ell.Sample.Output
		if len(ell.Sample.Input) > 50 {
			input = ell.Sample.Input[:50] + "…"
		}
		if len(ell.Sample.Expected) > 50 {
			expected = ell.Sample.Expected[:50] + "…"
		}
		if len(ell.Sample.Output) > 50 {
			output = ell.Sample.Output[:50] + "…"
		}

		var scoreChange string
		switch {
		case newScore:
			scoreChange = "🆕"
		case previousScore < ell.Result.Score:
			scoreChange = "↗️"
		case previousScore > ell.Result.Score:
			scoreChange = "↘️"
		case previousScore == ell.Result.Score:
			scoreChange = "➡️"
		}

		fmt.Printf("| %s | %s | %s | %s | %s | %.2f %v | %v |\n",
			ell.Name, input, expected, output, ell.Result.Type, ell.Result.Score, scoreChange, ell.Duration)

		err := h.Exec(ctx, `insert into evals (experiment, name, input, expected, output, type, score, duration) values (?, ?, ?, ?, ?, ?, ?, ?)`,
			*experiment, ell.Name, ell.Sample.Input, ell.Sample.Expected, ell.Sample.Output, ell.Result.Type, ell.Result.Score, ell.Duration)
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
