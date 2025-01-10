package text_test

import (
	"embed"
	"strings"
	"testing"
	"time"

	"maragu.dev/is"

	"maragu.dev/evals/internal/text"
)

//go:embed testdata
var testdata embed.FS

func TestParser_Parse(t *testing.T) {
	t.Run("can parse a complete log line", func(t *testing.T) {
		tests := []struct {
			path string
		}{
			{"one.txt"},
			{"two.txt"},
			{"three.txt"},
		}

		for _, test := range tests {
			t.Run(test.path, func(t *testing.T) {
				p := &text.Parser{}

				lines := getFileAsLines(t, test.path)

				var ell text.EvalLogLine

				for _, line := range lines {
					var ok bool
					ell, ok = p.Parse(line)
					if ok {
						break
					}
				}
				is.Equal(t, "ping", ell.Sample.Input)
				is.Equal(t, "pong", ell.Sample.Expected)
				is.Equal(t, "plong", ell.Sample.Output)
				is.Equal(t, 0.8, ell.Result.Score)
				is.Equal(t, "LevenshteinDistance", ell.Result.Type)
				is.Equal(t, time.Duration(1209), ell.Duration)
			})
		}
	})

	t.Run("can parse really long output", func(t *testing.T) {
		t.Skip()
		p := &text.Parser{}

		lines := getFileAsLines(t, "long.txt")

		var ell text.EvalLogLine
		var ok bool
		for _, line := range lines {
			ell, ok = p.Parse(line)
			if ok {
				break
			}
		}
		is.True(t, ok)
		is.Equal(t, "ping", ell.Sample.Input)
		is.Equal(t, "pong", ell.Sample.Expected)
		is.Equal(t, "plong", ell.Sample.Output)
		is.Equal(t, 0.8, ell.Result.Score)
		is.Equal(t, "LevenshteinDistance", ell.Result.Type)
		is.Equal(t, time.Duration(1209), ell.Duration)
	})
}

func getFileAsLines(t *testing.T, path string) []string {
	t.Helper()

	f, err := testdata.ReadFile("testdata/" + path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(f), "\n")
	var result []string
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	lines = result
	return lines
}
