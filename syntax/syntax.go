package syntax

import (
	"github.com/pachyderm/glob/syntax/ast"
	"github.com/pachyderm/glob/syntax/lexer"
)

func Parse(s string) (*ast.Node, error) {
	return ast.Parse(lexer.NewLexer(s))
}
