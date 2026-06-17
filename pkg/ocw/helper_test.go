package ocw

import (
	"regexp"
	"testing"
)

func TestNanoidAlphabet(t *testing.T) {
	valid := regexp.MustCompile(`^[a-z0-9-]+$`)
	if !valid.MatchString(NanoidAlphabet) {
		t.Errorf("NanoidAlphabet %q contains invalid characters (must be lowercase letters, digits, and hyphens only)", NanoidAlphabet)
	}
}
