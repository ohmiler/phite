package portable

import "testing"

func TestNameUsesOneCrossPlatformInvariant(t *testing.T) {
	for _, valid := range []string{"composer.phar", "third-party-notices-2.10.2.zip", "2.10.2", "A_1-b"} {
		if !Name(valid) {
			t.Errorf("expected %q to be portable", valid)
		}
	}
	for _, invalid := range []string{"", ".", "..", ".hidden", "trailing.", "trailing ", "a/b", `a\\b`, "a:b", "a?b", "a*b", `a"b`, "a<b", "a>b", "a|b", "a\x01b", "NUL", "nul.txt", "COM1.log", "LPT9"} {
		if Name(invalid) {
			t.Errorf("expected %q to be rejected", invalid)
		}
	}
}
