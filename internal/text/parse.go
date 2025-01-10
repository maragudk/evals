package text

import (
	"encoding/json"
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
var goTestOutputMatcher = regexp.MustCompile(`\s+[\w.]+:\d+:\s((?s:.+))`)

const (
	prefix = "EVALRESULT🌜"
	suffix = "🌛EVALRESULT"
)

type EvalLogLine struct {
	Name     string
	Sample   eval.Sample
	Result   eval.Result
	Duration time.Duration
}

type Parser struct {
	parsing bool
	lines   string
}

// Parse a line and return whether it's complete.
func (p *Parser) Parse(line string) (EvalLogLine, bool) {
	var gtl goTestLine
	if err := json.Unmarshal([]byte(line), &gtl); err != nil {
		panic("error unmarshalling line: " + err.Error())
	}

	if gtl.Action != "output" || !strings.HasPrefix(gtl.Test, "TestEval") {
		return EvalLogLine{}, false
	}

	matches := goTestOutputMatcher.FindStringSubmatch(gtl.Output)

	if len(matches) == 0 {
		return EvalLogLine{}, false
	}

	match := strings.TrimSpace(matches[1])

	if strings.HasPrefix(match, prefix) && strings.HasSuffix(match, suffix) {
		match = strings.TrimPrefix(match, prefix)
		match = strings.TrimSuffix(match, suffix)
		var ell EvalLogLine
		if err := json.Unmarshal([]byte(match), &ell); err != nil {
			panic(err)
		}
		return ell, true
	}

	// Beginning of an incomplete line
	if strings.HasPrefix(match, prefix) {
		p.parsing = true
		p.lines = match
		return EvalLogLine{}, false
	}

	if p.parsing {
		p.lines += match

		if strings.HasSuffix(match, suffix) {
			p.parsing = false

			p.lines = strings.TrimPrefix(p.lines, prefix)
			p.lines = strings.TrimSuffix(p.lines, suffix)

			var ell EvalLogLine
			if err := json.Unmarshal([]byte(p.lines), &ell); err != nil {
				panic(err)
			}
			ell.Name = gtl.Test
			return ell, true
		}
	}

	return EvalLogLine{}, false
}
