package compiler

// TODO use constructor with all matchers, and to their structs private
// TODO glue multiple Text nodes (like after QuoteMeta)

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pachyderm/glob/syntax/ast"
)

func meta(s interface{}) string {
	return regexp.QuoteMeta(fmt.Sprintf("%s", s))
}

func compileTreeChildren(tree *ast.Node, sep []rune, concatWith string) (string, error) {
	childRegex := make([]string, 0)
	for _, desc := range tree.Children {
		cr, err := Compile(desc, sep)
		if err != nil {
			return "", err
		}
		childRegex = append(childRegex, cr)
	}
	return strings.Join(childRegex, concatWith), nil
}

func Compile(tree *ast.Node, sep []rune) (string, error) {
	var err error
	regex := ""
	switch tree.Kind {
	case ast.KindAnyOf:
		if len(tree.Children) == 0 {
			return "", nil
		}
		anyOfRegex, err := compileTreeChildren(tree, sep, "|")
		if err != nil {
			return "", err
		}
		return "(?:" + anyOfRegex + ")", nil

	case ast.KindPattern:
		if len(tree.Children) == 0 {
			return "", nil
		}
		regex, err = compileTreeChildren(tree, sep, "")
		if err != nil {
			return "", err
		}

	case ast.KindCapture:
		if len(tree.Children) == 0 {
			return "", nil
		}
		captureRegex, err := compileTreeChildren(tree, sep, "|")
		if err != nil {
			return "", err
		}
		return "(" + captureRegex + ")", nil

	case ast.KindAny:
		regex = fmt.Sprintf("[^%v]*", meta(sep))

	case ast.KindSuper:
		regex = ".*"

	case ast.KindSingle:
		regex = fmt.Sprintf("[^%v]", meta(sep))

	case ast.KindNothing:
		regex = ""

	case ast.KindList:
		l := tree.Value.(ast.List)
		sign := ""
		if l.Not {
			sign = "^"
		}
		regex = fmt.Sprintf("[%v%v]", sign, meta(l.Chars))

	case ast.KindRange:
		r := tree.Value.(ast.Range)
		sign := ""
		if r.Not {
			sign = "^"
		}
		regex = fmt.Sprintf("[%v%v-%v]", sign, meta(r.Lo), meta(r.Hi))

	case ast.KindText:
		t := tree.Value.(ast.Text)
		regex = meta(t)

	default:
		return "", fmt.Errorf("could not compile tree: unknown node type")
	}

	return regex, nil
}
