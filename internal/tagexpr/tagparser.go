package tagexpr

import (
	"fmt"
	"strings"
	"unicode"
)

type namedTagExpr struct {
	exprSelector string
	expr         *Expr
}

const (
	tagOmit    = "-"
	tagOmitNil = "?"
)

func (f *fieldVM) parseExprs(tag string) error {
	switch tag {
	case tagOmit, tagOmitNil:
		f.tagOp = tag
		return nil
	}

	kvs, err := parseTag(tag)
	if err != nil {
		return err
	}
	exprSelectorPrefix := f.structField.Name

	for exprSelector, exprString := range kvs {
		expr, err := parseExpr(exprString)
		if err != nil {
			return err
		}
		if exprSelector == ExprNameSeparator {
			exprSelector = exprSelectorPrefix
		} else {
			exprSelector = exprSelectorPrefix + ExprNameSeparator + exprSelector
		}
		f.exprs[exprSelector] = expr
		f.origin.exprs[exprSelector] = expr
		f.origin.exprSelectorList = append(f.origin.exprSelectorList, exprSelector)
	}
	return nil
}

func parseTag(tag string) (map[string]string, error) {
	s := tag
	ptr := &s
	kvs := make(map[string]string)
	for {
		one, err := readOneExpr(ptr)
		if err != nil {
			return nil, err
		}
		if one == "" {
			return kvs, nil
		}
		key, val := splitExpr(one)
		if val == "" {
			return nil, fmt.Errorf("syntax error: %q expression string can not be empty", tag)
		}
		if _, ok := kvs[key]; ok {
			return nil, fmt.Errorf("syntax error: %q duplicate expression name %q", tag, key)
		}
		kvs[key] = val
	}
}

func splitExpr(one string) (key, val string) {
	one = strings.TrimSpace(one)
	if one == "" {
		return DefaultExprName, ""
	}
	var rs []rune
	for _, r := range one {
		if r == '@' ||
			r == '_' ||
			(r >= '0' && r <= '9') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') {
			rs = append(rs, r)
		} else {
			break
		}
	}
	key = string(rs)
	val = strings.TrimSpace(one[len(key):])
	if val == "" || val[0] != ':' {
		return DefaultExprName, one
	}
	val = val[1:]
	if key == "" {
		key = DefaultExprName
	}
	return key, val
}

func readOneExpr(tag *string) (string, error) {
	var s = *(trimRightSpace(trimLeftSpace(tag)))
	s = strings.TrimLeft(s, ";")
	if s == "" {
		return "", nil
	}
	if s[len(s)-1] != ';' {
		s += ";"
	}
	a := strings.SplitAfter(strings.Replace(s, "\\'", "##", -1), ";")
	var idx = -1
	var patch int
	for _, v := range a {
		idx += len(v)
		count := strings.Count(v, "'")
		if (count+patch)%2 == 0 {
			*tag = s[idx+1:]
			return s[:idx], nil
		}
		if count > 0 {
			patch++
		}
	}
	return "", fmt.Errorf("syntax error: %q unclosed single quote \"'\"", s)
}

func trimLeftSpace(p *string) *string {
	*p = strings.TrimLeftFunc(*p, unicode.IsSpace)
	return p
}

func trimRightSpace(p *string) *string {
	*p = strings.TrimRightFunc(*p, unicode.IsSpace)
	return p
}

func readPairedSymbol(p *string, left, right rune) *string {
	s := *p
	if len(s) == 0 || rune(s[0]) != left {
		return nil
	}
	s = s[1:]
	var last1 = left
	var last2 rune
	var leftLevel, rightLevel int
	var escapeIndexes = make(map[int]bool)
	var realEqual, escapeEqual bool
	for i, r := range s {
		if realEqual, escapeEqual = equalRune(right, r, last1, last2); realEqual {
			if leftLevel == rightLevel {
				*p = s[i+1:]
				var sub = make([]rune, 0, i)
				for k, v := range s[:i] {
					if !escapeIndexes[k] {
						sub = append(sub, v)
					}
				}
				s = string(sub)
				return &s
			}
			rightLevel++
		} else if escapeEqual {
			escapeIndexes[i-1] = true
		} else if realEqual, escapeEqual = equalRune(left, r, last1, last2); realEqual {
			leftLevel++
		} else if escapeEqual {
			escapeIndexes[i-1] = true
		}
		last2 = last1
		last1 = r
	}
	return nil
}

func equalRune(a, b, last1, last2 rune) (real, escape bool) {
	if a == b {
		real = last1 != '\\' || last2 == '\\'
		escape = last1 == '\\' && last2 != '\\'
	}
	return
}
