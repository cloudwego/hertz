package generator

import "testing"

func TestAppendMw(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		input    string
		want     string
	}{
		{"first", []string{}, "mw", "mw"},
		{"dup once", []string{"mw"}, "mw", "mw1"},
		{"several", []string{"mw", "mw1", "mw2"}, "mw", "mw3"},
		{"other existing", []string{"foo", "bar"}, "mw", "mw"},
		{"has mw0 present", []string{"mw0"}, "mw", "mw"},
		{"has mw and mw0", []string{"mw", "mw0"}, "mw", "mw1"},
		{"has mw, mw0, mw1", []string{"mw", "mw0", "mw1"}, "mw", "mw2"},
	}
	for _, tc := range cases {
		mwsCopy := append([]string(nil), tc.existing...)
		mws, out := appendMw(mwsCopy, tc.input)
		if out != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, out, tc.want)
		}
		if !stringsIncludes(mws, out) {
			t.Fatalf("%s: result %q not inserted into slice", tc.name, out)
		}
	}
}
