package glob

import (
	"time"

	"github.com/dlclark/regexp2"

	"github.com/pachyderm/glob/compiler"
	"github.com/pachyderm/glob/syntax"
)

// Glob represents compiled glob pattern.
type Glob struct {
	r *regexp2.Regexp
}

// Compile creates Glob for given pattern and strings (if any present after pattern) as separators.
// The pattern syntax is:
//
//    pattern:
//        { term }
//
//    term:
//        `*`         matches any sequence of non-separator characters
//        `**`        matches any sequence of characters
//        `?`         matches any single non-separator character
//        `[` [ `!` ] { character-range } `]`
//                    character class (must be non-empty)
//        `{` pattern-list `}`
//                    pattern alternatives
//        c           matches character c (c != `*`, `**`, `?`, `\`, `[`, `{`, `}`)
//        `\` c       matches character c
//
//    character-range:
//        c           matches character c (c != `\\`, `-`, `]`)
//        `\` c       matches character c
//        lo `-` hi   matches character c for lo <= c <= hi
//
//    pattern-list:
//        pattern { `,` pattern }
//                    comma-separated (without spaces) patterns
//
//    extended-glob:
//        `(` { `|` pattern } `)`
//        `@(` { `|` pattern } `)`
//                    capture one of pipe-separated subpatterns
//        `*(` { `|` pattern } `)`
//                    capture any number of of pipe-separated subpatterns
//        `+(` { `|` pattern } `)`
//                    capture one or more of of pipe-separated subpatterns
//        `?(` { `|` pattern } `)`
//                    capture zero or one of of pipe-separated subpatterns
//
func Compile(pattern string, separators ...rune) (*Glob, error) {
	ast, err := syntax.Parse(pattern)
	if err != nil {
		return nil, err
	}

	regex, err := compiler.Compile(ast, separators)
	if err != nil {
		return nil, err
	}
	r, err := regexp2.Compile(regex, 0)
	r.MatchTimeout = time.Minute * 5 // if it takes more than 5minutes to match a glob, something is very wrong
	if err != nil {
		return nil, err
	}
	return &Glob{r: r}, nil
}

// MustCompile is the same as Compile, except that if Compile returns error, this will panic
func MustCompile(pattern string, separators ...rune) *Glob {
	g, err := Compile(pattern, separators...)
	if err != nil {
		panic(err)
	}
	return g
}

func (g *Glob) Match(fixture string) bool {
	m, err := g.r.MatchString(fixture)
	if err != nil {
		// this is taking longer than 5 minutes, so something is seriously wrong
		panic(err)
	}
	return m
}

func (g *Glob) Capture(fixture string) []string {
	m, err := g.r.FindStringMatch(fixture)
	if err != nil {
		// this is taking longer than 5 minutes, so something is seriously wrong
		panic(err)
	}
	if m == nil {
		return nil
	}
	groups := m.Groups()
	captures := make([]string, 0, len(groups))

	for _, gp := range groups {
		captures = append(captures, gp.Capture.String())
	}
	return captures
}

// QuoteMeta returns a string that quotes all glob pattern meta characters
// inside the argument text; For example, QuoteMeta(`*(foo*)`) returns `\*\(foo\*\)`.
func QuoteMeta(s string) string {
	b := make([]byte, 2*len(s))

	// a byte loop is correct because all meta characters are ASCII
	j := 0
	for i := 0; i < len(s); i++ {
		if syntax.Special(s[i]) {
			b[j] = '\\'
			j++
		}
		b[j] = s[i]
		j++
	}

	return string(b[0:j])
}
