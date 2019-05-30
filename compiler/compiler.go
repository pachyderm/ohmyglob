package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pachyderm/glob/syntax/ast"
)

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
	case ast.KindAnyOf:
		if len(tree.Children) == 0 {
			return "", nil
		}
		anyOfRegex, err := compileChildren(tree, sep, "|")
		if err != nil {
			return "", err
		}
		return "(?:" + anyOfRegex + ")", nil

	case ast.KindPattern:
		if len(tree.Children) == 0 {
			return "", nil
		}
		regex, err = compileChildren(tree, sep, "")
		if err != nil {
			return "", err
		}

	case ast.KindCapture:
		if len(tree.Children) == 0 {
			return "", nil
		}
		c := tree.Value.(ast.Capture)
		captureRegex, err := compileChildren(tree, sep, "|")
		if err != nil {
			return "", err
		}
		captureRegex = "(" + captureRegex + ")"
		switch c.Quantifier {
		case "*":
			return captureRegex + "*", nil
		case "?":
			return captureRegex + "?", nil
		case "+":
			return captureRegex + "+", nil
		case "@":
			return captureRegex, nil
		}
		return "", fmt.Errorf("unimplemented quatifier %v", c.Quantifier)

	case ast.KindAny:
		if len(sep) == 0 {
			regex = ".*"
		} else {
			regex = fmt.Sprintf("[^%v]*", meta(string(sep)))
		}

	case ast.KindSuper:
		regex = ".*"

	case ast.KindSingle:
		if len(sep) == 0 {
			regex = "."
		} else {
			regex = fmt.Sprintf("[^%v]", meta(string(sep)))
		}

	case ast.KindNothing:
		regex = ""

	case ast.KindList:
		l := tree.Value.(ast.List)
		sign := ""
		if l.Not {
			sign = "^"
		}
		regex = fmt.Sprintf("[%v%v]", sign, meta(string(l.Chars)))

	case ast.KindRange:
		r := tree.Value.(ast.Range)
		sign := ""
		if r.Not {
			sign = "^"
		}
		regex = fmt.Sprintf("[%v%v-%v]", sign, meta(string(r.Lo)), meta(string(r.Hi)))

	case ast.KindText:
		t := tree.Value.(ast.Text)
		regex = meta(t.Text)

	default:
		return "", fmt.Errorf("could not compile tree: unknown node type")
	}

	return regex, nil
}

func Compile(tree *ast.Node, sep []rune) (string, error) {
	regex, err := compile(tree, sep)
	if err != nil {
		return "", err
	}
	return "\\A" + regex + "\\z", nil
}
