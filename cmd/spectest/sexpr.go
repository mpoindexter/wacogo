package main

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// SExpr represents an s-expression node
type SExpr struct {
	Type     ExprType
	Value    string
	Quoted   bool // true if this atom was originally a quoted string
	Children []*SExpr
}

// ExprType represents the type of s-expression node
type ExprType int

const (
	ExprAtom ExprType = iota
	ExprList
)

// Parser handles s-expression parsing
type Parser struct {
	input []rune
	pos   int
}

// NewParser creates a new s-expression parser
func NewParser(input string) *Parser {
	return &Parser{
		input: []rune(input),
		pos:   0,
	}
}

// Parse parses the input into a list of s-expressions
func (p *Parser) Parse() ([]*SExpr, error) {
	var exprs []*SExpr

	for p.pos < len(p.input) {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			break
		}

		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
	}

	return exprs, nil
}

// parseExpr parses a single s-expression
func (p *Parser) parseExpr() (*SExpr, error) {
	p.skipWhitespace()

	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of input")
	}

	if p.input[p.pos] == '(' {
		return p.parseList()
	}

	return p.parseAtom()
}

// parseList parses a list s-expression
func (p *Parser) parseList() (*SExpr, error) {
	if p.input[p.pos] != '(' {
		return nil, fmt.Errorf("expected '(' at position %d", p.pos)
	}
	p.pos++ // consume '('

	expr := &SExpr{
		Type:     ExprList,
		Children: []*SExpr{},
	}

	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unexpected end of input, expected ')'")
		}

		if p.input[p.pos] == ')' {
			p.pos++ // consume ')'
			break
		}

		child, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		expr.Children = append(expr.Children, child)
	}

	return expr, nil
}

// parseAtom parses an atom (symbol, number, string)
func (p *Parser) parseAtom() (*SExpr, error) {
	if p.input[p.pos] == '"' {
		return p.parseString()
	}

	start := p.pos
	for p.pos < len(p.input) && !unicode.IsSpace(p.input[p.pos]) &&
		p.input[p.pos] != '(' && p.input[p.pos] != ')' {
		p.pos++
	}

	if start == p.pos {
		return nil, fmt.Errorf("expected atom at position %d", p.pos)
	}

	return &SExpr{
		Type:  ExprAtom,
		Value: string(p.input[start:p.pos]),
	}, nil
}

// parseString parses a quoted string
func (p *Parser) parseString() (*SExpr, error) {
	if p.input[p.pos] != '"' {
		return nil, fmt.Errorf("expected '\"' at position %d", p.pos)
	}
	p.pos++ // consume opening quote

	var value strings.Builder
	for p.pos < len(p.input) {
		if p.input[p.pos] == '"' {
			p.pos++ // consume closing quote
			return &SExpr{
				Type:   ExprAtom,
				Value:  value.String(),
				Quoted: true,
			}, nil
		}

		if p.input[p.pos] == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			switch p.input[p.pos] {
			case 'n':
				value.WriteRune('\n')
			case 't':
				value.WriteRune('\t')
			case '\\':
				value.WriteRune('\\')
			case '"':
				value.WriteRune('"')
			default:
				value.WriteRune(p.input[p.pos])
			}
			p.pos++
		} else {
			value.WriteRune(p.input[p.pos])
			p.pos++
		}
	}

	return nil, fmt.Errorf("unterminated string")
}

// skipWhitespace skips whitespace and comments
func (p *Parser) skipWhitespace() {
	for p.pos < len(p.input) {
		if unicode.IsSpace(p.input[p.pos]) {
			p.pos++
		} else if p.input[p.pos] == ';' {
			// Skip comment until end of line
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

// Dump returns the s-expression as a string
func (e *SExpr) Dump() string {
	var sb strings.Builder
	e.dumpTo(&sb)
	return sb.String()
}

// dumpTo writes the s-expression to a strings.Builder
func (e *SExpr) dumpTo(sb *strings.Builder) {
	if e.Type == ExprAtom {
		// Use the Quoted field to determine if we should quote
		if e.Quoted {
			sb.WriteRune('"')
			for _, ch := range e.Value {
				switch ch {
				case '"':
					sb.WriteString("\\\"")
				case '\\':
					sb.WriteString("\\\\")
				case '\n':
					sb.WriteString("\\n")
				case '\t':
					sb.WriteString("\\t")
				default:
					sb.WriteRune(ch)
				}
			}
			sb.WriteRune('"')
		} else {
			sb.WriteString(e.Value)
		}
	} else {
		sb.WriteRune('(')
		for i, child := range e.Children {
			if i > 0 {
				sb.WriteRune(' ')
			}
			child.dumpTo(sb)
		}
		sb.WriteRune(')')
	}
}

// needsQuoting checks if a string value needs to be quoted
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, ch := range s {
		if unicode.IsSpace(ch) || ch == '(' || ch == ')' || ch == '"' || ch == '\\' {
			return true
		}
	}
	return false
}

// Walk traverses the tree calling fn for each node
func (e *SExpr) Walk(fn func(*SExpr) bool) {
	if !fn(e) {
		return
	}
	if e.Type == ExprList {
		for _, child := range e.Children {
			child.Walk(fn)
		}
	}
}

// RemoveChild removes a child from a list expression
func (e *SExpr) RemoveChild(index int) error {
	if e.Type != ExprList {
		return fmt.Errorf("cannot remove child from atom")
	}
	if index < 0 || index >= len(e.Children) {
		return fmt.Errorf("index out of range: %d", index)
	}
	e.Children = append(e.Children[:index], e.Children[index+1:]...)
	return nil
}

// RemoveChildIf removes all children matching the predicate
func (e *SExpr) RemoveChildIf(pred func(*SExpr) bool) int {
	if e.Type != ExprList {
		return 0
	}

	removed := 0
	newChildren := make([]*SExpr, 0, len(e.Children))
	for _, child := range e.Children {
		if !pred(child) {
			newChildren = append(newChildren, child)
		} else {
			removed++
		}
	}
	e.Children = newChildren
	return removed
}

// AddChild adds a child to a list expression
func (e *SExpr) AddChild(child *SExpr) error {
	if e.Type != ExprList {
		return fmt.Errorf("cannot add child to atom")
	}
	e.Children = append(e.Children, child)
	return nil
}

// InsertChild inserts a child at the specified index
func (e *SExpr) InsertChild(index int, child *SExpr) error {
	if e.Type != ExprList {
		return fmt.Errorf("cannot insert child into atom")
	}
	if index < 0 || index > len(e.Children) {
		return fmt.Errorf("index out of range: %d", index)
	}

	e.Children = append(e.Children[:index], append([]*SExpr{child}, e.Children[index:]...)...)
	return nil
}

// Clone creates a deep copy of the s-expression
func (e *SExpr) Clone() *SExpr {
	clone := &SExpr{
		Type:   e.Type,
		Value:  e.Value,
		Quoted: e.Quoted,
	}

	if e.Type == ExprList {
		clone.Children = make([]*SExpr, len(e.Children))
		for i, child := range e.Children {
			clone.Children[i] = child.Clone()
		}
	}

	return clone
}

// FindAll returns all nodes matching the predicate
func (e *SExpr) FindAll(pred func(*SExpr) bool) []*SExpr {
	var results []*SExpr
	e.Walk(func(node *SExpr) bool {
		if pred(node) {
			results = append(results, node)
		}
		return true
	})
	return results
}

// PrettyPrint writes a formatted version of the s-expression
func (e *SExpr) PrettyPrint(w io.Writer, indent int) {
	prefix := strings.Repeat("  ", indent)

	if e.Type == ExprAtom {
		fmt.Fprintf(w, "%s%s\n", prefix, e.Value)
	} else {
		fmt.Fprintf(w, "%s(\n", prefix)
		for _, child := range e.Children {
			child.PrettyPrint(w, indent+1)
		}
		fmt.Fprintf(w, "%s)\n", prefix)
	}
}

// IsAtom returns true if this is an atom
func (e *SExpr) IsAtom() bool {
	return e.Type == ExprAtom
}

// IsList returns true if this is a list
func (e *SExpr) IsList() bool {
	return e.Type == ExprList
}

// AtomValue returns the atom value or empty string if not an atom
func (e *SExpr) AtomValue() string {
	if e.Type == ExprAtom {
		return e.Value
	}
	return ""
}
