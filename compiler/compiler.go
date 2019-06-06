package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pachyderm/glob/syntax/ast"
)

// these dummy strings are not valid UTF-8, so we can use them without worrying about them also matching legitmate input
const (
	closeNegDummy = string(0xffff)
	boundaryDummy = string(0xfffe)
)

func dot(sep []rune) string {
	meta := regexp.QuoteMeta
	if len(sep) == 0 {
		return "."
	}
	return fmt.Sprintf("[^%v]", meta(string(sep)))
}

func compileChildren(tree *ast.Node, sep []rune, concatWith string) (string, error) {
	childRegex := make([]string, 0)
	for _, desc := range tree.Children {
		cr, err := compile(desc, sep)
		if err != nil {
			return "", err
		}
		childRegex = append(childRegex, cr)
	}
	return strings.Join(childRegex, concatWith), nil
}

func compile(tree *ast.Node, sep []rune) (string, error) {
	var err error
	regex := ""
	meta := regexp.QuoteMeta
	switch tree.Kind {
	// stuff between braces becomes a non-capturing group OR'd together
	case ast.KindAnyOf:
		if len(tree.Children) == 0 {
			return "", nil
		}
		anyOfRegex, err := compileChildren(tree, sep, "|")
		if err != nil {
			return "", err
		}
		return "(?:" + anyOfRegex + ")", nil

	// subexpresions are simply concatenated
	case ast.KindPattern:
		if len(tree.Children) == 0 {
			return "", nil
		}
		regex, err = compileChildren(tree, sep, "")
		if err != nil {
			return "", err
		}
		// only the last negative of a subexpression should have the closeNegDummy
		notNum := strings.Count(regex, closeNegDummy)
		regex = strings.Replace(regex, closeNegDummy, "", notNum-1)

	// capture groups become capture groups, with the stuff in them OR'd together
	case ast.KindCapture:
		if len(tree.Children) == 0 {
			return "", nil
		}
		c := tree.Value.(ast.Capture)
		captureRegex, err := compileChildren(tree, sep, "|")
		if err != nil {
			return "", err
		}
		switch c.Quantifier {
		case "*":
			return "((?:" + captureRegex + ")*)", nil
		case "?":
			return "((?:" + captureRegex + ")?)", nil
		case "+":
			return "((?:" + captureRegex + ")+)", nil
		case "@":
			return "(" + captureRegex + ")", nil
		case "!":
			// not only does a negation capture require PCRE
			// it also requires a complicated function to determine how aggressively it should match
			// this requires global information, so we cannot do it here, instead we insert a `closeNegDummy`
			// to mark the spot where we might potentially need this
			return "((?:(?!(?:" + captureRegex + fmt.Sprintf("%v))%v*))", closeNegDummy, dot(sep)), nil
		}

		return "", fmt.Errorf("unimplemented quatifier %v", c.Quantifier)

	// glob `*` essentially becomes `.*`, but excluding any separators
	case ast.KindAny:
		// `*` is more aggressive than
		regex = dot(sep) + "*" + boundaryDummy

	// glob `**` is just `.*`
	case ast.KindSuper:
		regex = ".*" + boundaryDummy

	// glob `?` essentially becomes `.`, but excluding any separators
	case ast.KindSingle:
		regex = dot(sep)

	case ast.KindNothing:
		regex = ""

	// stuff in a list e.g. `[abcd]` is handled the same way by regexp
	case ast.KindList:
		l := tree.Value.(ast.List)
		sign := ""
		if l.Not {
			sign = "^"
		}
		regex = fmt.Sprintf("[%v%v]", sign, meta(string(l.Chars)))

	// POSIX classes like `[[:alpha:]]` are handled the same way by regexp
	case ast.KindPOSIX:
		p := tree.Value.(ast.POSIX)
		sign := ""
		if p.Not {
			sign = "^"
		}
		regex = fmt.Sprintf("[%v[:%v:]]", sign, meta(string(p.Class)))

	// stuff in a range e.g. `[a-d]` is handled the same way by regexp
	case ast.KindRange:
		r := tree.Value.(ast.Range)
		sign := ""
		if r.Not {
			sign = "^"
		}
		regex = fmt.Sprintf("[%v%v-%v]", sign, meta(string(r.Lo)), meta(string(r.Hi)))

	// text just matches text, after we escape any special regexp chars
	case ast.KindText:
		t := tree.Value.(ast.Text)
		regex = meta(t.Text) + boundaryDummy
		fmt.Println(t.Text)

	default:
		return "", fmt.Errorf("could not compile tree: unknown node type")
	}

	return regex, nil
}

// Compile takes a glob AST, and converts it into a regular expression
// Any separator characters (typically the path directory char: `/` or `\`)
// are passed in to allow the compiler to handle them correctly
func Compile(tree *ast.Node, sep []rune) (string, error) {
	regex, err := compile(tree, sep)
	if err != nil {
		return "", err
	}

	index := strings.Index(regex, closeNegDummy)
	if index > 0 {
		index++
		extraNum :=
			// (strings.Count(regex[index:], ")"+boundaryDummy)-
			// 	strings.Count(regex[index:], "\\)"+boundaryDummy)) <= 0 &&
			strings.Contains(regex[index:], boundaryDummy)

		if extraNum {
			regex = strings.Replace(regex, closeNegDummy, "", -1)
		} else {
			regex = strings.Replace(regex, closeNegDummy, "$", -1)
		}
	}
	regex = strings.Replace(regex, boundaryDummy, "", -1)
	// globs are expected to match against the whole thing
	return "\\A" + regex + "\\z", nil
}
