package upgrade

import "testing"

func TestVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		newer           bool
		invalid         bool
	}{
		{"0.1.1", "0.1.0", true, false},
		{"0.1.0", "0.1.0", false, false},
		{"0.1.0", "0.2.0", false, false},
		{"2.0.0", "1.99.999", true, false},
		{"1.2.3-rc1", "1.2.2", false, true},
		{"1.2", "1.0.0", false, true},
		{"1.2.3", "dev", false, true},
	} {
		got, err := newerVersion(tc.latest, tc.current)
		if (err != nil) != tc.invalid || (!tc.invalid && got != tc.newer) {
			t.Errorf("newerVersion(%q, %q) = %v, %v", tc.latest, tc.current, got, err)
		}
	}
}

func TestReleaseRedirectValidation(t *testing.T) {
	for _, tc := range []struct {
		location, want string
	}{
		{"https://github.com/amxv/dm-dot/releases/tag/v1.2.3", "1.2.3"},
		{"/amxv/dm-dot/releases/tag/v2.3.4", "2.3.4"},
		{"https://evil.example/amxv/dm-dot/releases/tag/v1.2.3", ""},
		{"https://github.com/other/dm-dot/releases/tag/v1.2.3", ""},
		{"https://github.com/amxv/dm-dot/releases/tag/v1.2.3?secret=1", ""},
		{"https://github.com/amxv/dm-dot/releases/tag/v1.2.3-rc1", ""},
		{"https://github.com/amxv/dm-dot/releases/tag/v1.2.3/extra", ""},
	} {
		got, err := tagFromLocation(tc.location)
		if got != tc.want || ((err != nil) != (tc.want == "")) {
			t.Errorf("tagFromLocation(%q) = %q, %v, want %q", tc.location, got, err, tc.want)
		}
	}
}
