package ast

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pachyderm/glob/syntax/lexer"
)

type CompilerToUse int

const (
	Regexp CompilerToUse = iota
	PCRE
)

type Lexer interface {
	Next() lexer.Token
}

type parseFn func(*Node, CompilerToUse, Lexer) (parseFn, *Node, CompilerToUse, error)

func Parse(lexer Lexer) (*Node, CompilerToUse, error) {
	var parser parseFn

	root := NewNode(KindPattern, nil)

	var (
		tree *Node
		err  error
	)
	comp := Regexp
	for parser, tree = parserMain, root; parser != nil; {
		parser, tree, comp, err = parser(tree, comp, lexer)
		if err != nil {
			return nil, comp, err
		}
	}

	return root, comp, nil
}

func parserMain(tree *Node, comp CompilerToUse, lex Lexer) (parseFn, *Node, CompilerToUse, error) {
	for {
		token := lex.Next()
		switch token.Type {
		case lexer.EOF:
			return nil, tree, comp, nil

		case lexer.Error:
			return nil, tree, comp, errors.New(token.Raw)

		case lexer.Text:
			Insert(tree, NewNode(KindText, Text{Text: token.Raw}))
			return parserMain, tree, comp, nil

		case lexer.Any:
			Insert(tree, NewNode(KindAny, nil))
			return parserMain, tree, comp, nil

		case lexer.Super:
			Insert(tree, NewNode(KindSuper, nil))
			return parserMain, tree, comp, nil

		case lexer.Single:
			Insert(tree, NewNode(KindSingle, nil))
			return parserMain, tree, comp, nil

		case lexer.RangeOpen:
			return parserRange, tree, comp, nil

		case lexer.TermsOpen:
			a := NewNode(KindAnyOf, nil)
			Insert(tree, a)

			p := NewNode(KindPattern, nil)
			Insert(a, p)

			return parserMain, p, comp, nil

		case lexer.CaptureOpen:
			if strings.ContainsAny(token.Raw, "!") {
				comp = PCRE
			}
			a := NewNode(KindCapture, Capture{token.Raw[:1]})
			Insert(tree, a)

			p := NewNode(KindPattern, nil)
			Insert(a, p)

			return parserMain, p, comp, nil

		case lexer.Separator:
			p := NewNode(KindPattern, nil)
			Insert(tree.Parent, p)

			return parserMain, p, comp, nil

		case lexer.TermsClose:
			return parserMain, tree.Parent.Parent, comp, nil

		case lexer.CaptureClose:
			return parserMain, tree.Parent.Parent, comp, nil

		default:
			return nil, tree, comp, fmt.Errorf("unexpected token: %s", token)
		}
	}
	return nil, tree, comp, fmt.Errorf("unknown error")
}

func parserRange(tree *Node, comp CompilerToUse, lex Lexer) (parseFn, *Node, CompilerToUse, error) {
	var (
		not   bool
		lo    rune
		hi    rune
		chars string
	)
	for {
		token := lex.Next()
		switch token.Type {
		case lexer.EOF:
			return nil, tree, comp, errors.New("unexpected end")

		case lexer.Error:
			return nil, tree, comp, errors.New(token.Raw)

		case lexer.Not:
			not = true

		case lexer.RangeLo:
			r, w := utf8.DecodeRuneInString(token.Raw)
			if len(token.Raw) > w {
				return nil, tree, comp, fmt.Errorf("unexpected length of lo character")
			}
			lo = r

		case lexer.RangeBetween:
			// do nothing

		case lexer.RangeHi:
			r, w := utf8.DecodeRuneInString(token.Raw)
			if len(token.Raw) > w {
				return nil, tree, comp, fmt.Errorf("unexpected length of lo character")
			}

			hi = r

			if hi < lo {
				return nil, tree, comp, fmt.Errorf("hi character '%s' should be greater than lo '%s'", string(hi), string(lo))
			}

		case lexer.Text:
			chars = token.Raw

		case lexer.RangeClose:
			isRange := lo != 0 && hi != 0
			isChars := chars != ""
			isPOSIX := false
			if len(chars) >= 2 && chars[:1] == ":" && chars[len(chars)-1:] == ":" {
				isPOSIX = true
			}

			if isChars == isRange {
				return nil, tree, comp, fmt.Errorf("could not parse range")
			}

			if isPOSIX {
				Insert(tree, NewNode(KindPOSIX, POSIX{
					Not:   strings.ContainsAny(chars, "^!") || not,
					Class: strings.Trim(chars, "[:]^!"),
				}))
			} else if isRange {
				Insert(tree, NewNode(KindRange, Range{
					Lo:  lo,
					Hi:  hi,
					Not: not,
				}))
			} else {
				Insert(tree, NewNode(KindList, List{
					Chars: chars,
					Not:   not,
				}))
			}

			return parserMain, tree, comp, nil
		}
	}
}
