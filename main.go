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
	"time"

	"maragu.dev/evals/internal/sql"
)

type evalLogLine struct {
	Name     string
	Sample   Sample
	Result   Result
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

	var score Score
	var duration time.Duration
	var n int

	for scanner.Scan() {
		var ell evalLogLine
		b := scanner.Bytes()
		if err := json.Unmarshal(b, &ell); err != nil {
			return fmt.Errorf("error unmarshalling line: %w", err)
		}

		var previousScore Score
		var newScore bool
		if err := h.Get(ctx, &previousScore, `select score from evals where name = ? and type = ? order by experiment desc limit 1`, ell.Name, ell.Result.Type); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("error getting previous score from database: %w", err)
			}
			newScore = true
		}

		if n == 0 {
			fmt.Println("| Name | Type | Score | Duration |")
			fmt.Println("| --- | --- | --: | --: |")
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

		fmt.Printf("| %s | %s | %.2f %v | %v |\n", ell.Name, ell.Result.Type, ell.Result.Score, scoreChange, ell.Duration)

		err := h.Exec(ctx, `insert into evals (experiment, name, input, expected, output, type, score, duration) values (?, ?, ?, ?, ?, ?, ?, ?)`,
			*experiment, ell.Name, ell.Sample.Input, ell.Sample.Expected, ell.Sample.Output, ell.Result.Type, ell.Result.Score, ell.Duration)
		if err != nil {
			return fmt.Errorf("error inserting eval into database: %w", err)
		}

		score += ell.Result.Score
		duration += ell.Duration
		n++
	}

	if n > 0 {
		// Print table footer with total score
		fmt.Printf("| **Total** | | | | | **%.2f** | **%v** |\n", float64(score)/float64(n), duration)
	}

	return nil
}
