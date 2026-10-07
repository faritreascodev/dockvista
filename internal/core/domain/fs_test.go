package domain

import "testing"

func TestCleanContainerPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"", "/", false},
		{"/", "/", false},
		{" /app ", "/app", false},
		{"/foo/../bar", "/bar", false},
		{"/foo/../../etc", "/etc", false},
		{"/./run", "/run", false},
		{"app", "", true},
		{"./app", "", true},
		{"/foo\x00bar", "", true},
	}
	for _, tc := range cases {
		got, err := CleanContainerPath(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("CleanContainerPath(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("CleanContainerPath(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("CleanContainerPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
