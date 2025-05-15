package generator

import (
	"regexp"
	"testing"
)

var jsonCheck = regexp.MustCompile(`(?i:(application|text)/(json|.*\+json|json\-.*)(;\s*|$))`)

func Test_JSONChecker(t *testing.T) {
	ct := []string{"application/json", "application/json;", "application/json; ", "application/json;charset=UTF-8", "application/json; charset=UTF-8"}

	for _, c := range ct {
		if jsonCheck.MatchString(c) != true {
			t.Errorf("%s is json content-type, but get false", c)
		}
	}
}
