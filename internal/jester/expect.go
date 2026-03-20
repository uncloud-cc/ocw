package jester

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-cmp/cmp"
	"github.com/muesli/termenv"
)

type Assertion struct {
	ActualValue any
	Testname    string
	T           *testing.T
}

func Expect(t *testing.T, actualValue any) *Assertion {
	t.Helper()

	return &Assertion{
		ActualValue: actualValue,
		Testname:    t.Name(),
		T:           t,
	}
}

// -------------------------------------------
// Matchers
// -------------------------------------------

func (a *Assertion) ToBe(expectedValue any) {
	a.T.Helper()

	if diff := cmp.Diff(expectedValue, a.ActualValue); diff != "" {
		a.renderFailedDiffOutput(expectedValue, diff, "Assertion error: Expected %v to be %v")
	}
}

func (a *Assertion) NotToBe(nonExpectedValue any) {
	a.T.Helper()

	if diff := cmp.Diff(nonExpectedValue, a.ActualValue); diff == "" {
		a.renderFailedDiffOutput(nil, diff, "Assertion error: Expected %v not to be %v")
	}
}

func (a *Assertion) ToBeNil() {
	a.T.Helper()

	if diff := cmp.Diff(nil, a.ActualValue); diff != "" {
		a.renderFailedDiffOutput(nil, diff, "Assertion error: Expected %v to be %v")
	}
}

func (a *Assertion) NotToBeNil() {
	a.T.Helper()

	if diff := cmp.Diff(nil, a.ActualValue); diff == "" {
		a.renderFailedDiffOutput(nil, diff, "Assertion error: Expected %v not to be %v")
	}
}

func (a *Assertion) ToMatchJSON(expectedJSON string) {
	a.T.Helper()

	// Parse expected JSON string
	var expectedValue any
	if err := json.Unmarshal([]byte(expectedJSON), &expectedValue); err != nil {
		a.T.Fatalf("Failed to parse expected JSON: %v\nJSON: %s", err, expectedJSON)
		return
	}

	// Parse actual value (could be string, []byte, or already parsed)
	var actualValue any
	switch v := a.ActualValue.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &actualValue); err != nil {
			a.T.Fatalf("Failed to parse actual JSON string: %v\nJSON: %s", err, v)
			return
		}
	case []byte:
		if err := json.Unmarshal(v, &actualValue); err != nil {
			a.T.Fatalf("Failed to parse actual JSON bytes: %v\nJSON: %s", err, string(v))
			return
		}
	default:
		// Already parsed or other type - use as is
		actualValue = v
	}

	// Compare the parsed values
	if diff := cmp.Diff(expectedValue, actualValue); diff != "" {
		a.renderFailedDiffOutput(expectedValue, diff, "Assertion error: Expected %v to be %v")
	}
}

func (a *Assertion) ToContainJSON(expectedJSON string) {
	a.T.Helper()

	// Parse expected JSON string
	var expectedValue any
	if err := json.Unmarshal([]byte(expectedJSON), &expectedValue); err != nil {
		a.T.Fatalf("Failed to parse expected JSON: %v\nJSON: %s", err, expectedJSON)
		return
	}

	// Parse actual value (could be string, []byte, or already parsed)
	var actualValue any
	switch v := a.ActualValue.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &actualValue); err != nil {
			a.T.Fatalf("Failed to parse actual JSON string: %v\nJSON: %s", err, v)
			return
		}
	case []byte:
		if err := json.Unmarshal(v, &actualValue); err != nil {
			a.T.Fatalf("Failed to parse actual JSON bytes: %v\nJSON: %s", err, string(v))
			return
		}
	default:
		// Already parsed or other type - use as is
		actualValue = v
	}

	// Filter actual to only include keys from expected, then compare
	filteredActual := filterToExpectedKeys(expectedValue, actualValue)
	if diff := cmp.Diff(expectedValue, filteredActual); diff != "" {
		a.renderFailedDiffOutput(expectedValue, diff, "Assertion error: Expected JSON to contain all fields from expected")
	}
}

// TODO: Make the string parsing more robust and universal
// Example: When calling err.Error() but the error is nil, the test suite immediately throws a nil pointer dereference error (ugly).
// Instead what I would love to have is a situation where the matcher tries to parse the error to string and notices that the error is nil (Expected xxx to be a string, but is nil instead)
func (a *Assertion) ToMatchPattern(pattern string) {
	a.T.Helper()

	// Ensure actual value is a string
	actualStr, ok := a.ActualValue.(string)
	if !ok {
		a.T.Fatalf("ToMatchPattern expects a string, got %T", a.ActualValue)
		return
	}

	// Compile and match the regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		a.T.Fatalf("Invalid regex pattern: %v\nPattern: %s", err, pattern)
		return
	}

	if !re.MatchString(actualStr) {
		diff := fmt.Sprintf("  string(\n-       /%s/,\n+       %q,\n  )", pattern, actualStr)
		a.renderFailedDiffOutput(pattern, diff, "Assertion error: Expected string to match pattern /%s/")
	}
}

// -------------------------------------------
// Render helpers
// -------------------------------------------

func (a *Assertion) renderFailedDiffOutput(expectedValue any, diff, errorMsg string) {
	a.T.Helper()

	// Force color output
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Vitest-style adaptive colors (light/dark terminal support)
	failRed := lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#F14C4C"}
	failBgRed := lipgloss.AdaptiveColor{Light: "#DC3545", Dark: "#CC0000"}
	successGreen := lipgloss.AdaptiveColor{Light: "#4E9A06", Dark: "#73C991"}
	dimGray := lipgloss.AdaptiveColor{Light: "#6A737D", Dark: "#8B949E"}

	// FAIL badge style (red background, white text)
	failStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(failBgRed).
		PaddingLeft(1).
		PaddingRight(1)

	// Error message style (red text)
	errorStyle := lipgloss.NewStyle().Foreground(failRed)

	// Test name style (dim gray)
	testNameStyle := lipgloss.NewStyle().Foreground(dimGray)

	// Diff legend styles
	expectedStyle := lipgloss.NewStyle().Foreground(successGreen)
	actualStyle := lipgloss.NewStyle().Foreground(failRed)

	// Prepare output
	failBadge := failStyle.Render(" FAIL ")
	failStatement := errorStyle.Render(fmt.Sprintf(errorMsg, expectedValue, a.ActualValue))
	failingTest := testNameStyle.Render(
		renderHumanReadableTestName(a.Testname),
	)

	diffHeader := fmt.Sprintf("%s %s\n%s", failBadge, failStatement, failingTest)
	diffLegend := renderExpectedActual("- Expected\n+ Actual", expectedStyle, actualStyle)
	coloredDiff := renderExpectedActual(diff, expectedStyle, actualStyle)

	// Actually output the whole thing
	a.T.Errorf("\n\n%s\n\n%s\n\n%s", diffHeader, diffLegend, coloredDiff)
}

func renderExpectedActual(rawString string, expectedStyle, actualStyle lipgloss.Style) string {
	// Normalize Windows line endings so prefix checks work consistently.
	rawString = strings.ReplaceAll(rawString, "\r\n", "\n")

	lines := strings.Split(rawString, "\n")
	var b strings.Builder
	b.Grow(len(rawString) + len(lines)*8) // small cushion for ANSI codes

	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "-"):
			b.WriteString(expectedStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(actualStyle.Render(line))
		default:
			b.WriteString(line)
		}

		// Re-add newline except after the last line to preserve original shape.
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func renderHumanReadableTestName(rawTestName string) string {
	withSpaces := strings.ReplaceAll(rawTestName, "_", " ")
	withoutSlashes := strings.ReplaceAll(withSpaces, "/", " > ")
	return withoutSlashes
}

// -------------------------------------------
// JSON partial matching helpers
// -------------------------------------------

// filterToExpectedKeys recursively filters actual to only include keys present in expected
func filterToExpectedKeys(expected, actual any) any {
	if expected == nil || actual == nil {
		return actual
	}

	expectedMap, expectedIsMap := expected.(map[string]any)
	actualMap, actualIsMap := actual.(map[string]any)

	// If expected is a map, filter actual map to only include expected keys
	if expectedIsMap && actualIsMap {
		filtered := make(map[string]any)
		for key, expectedVal := range expectedMap {
			if actualVal, exists := actualMap[key]; exists {
				// Recursively filter nested structures
				filtered[key] = filterToExpectedKeys(expectedVal, actualVal)
			}
		}
		return filtered
	}

	// If expected is a slice, filter each element
	expectedSlice, expectedIsSlice := expected.([]any)
	actualSlice, actualIsSlice := actual.([]any)

	if expectedIsSlice && actualIsSlice {
		filtered := make([]any, 0, len(expectedSlice))
		minLen := len(expectedSlice)
		if len(actualSlice) < minLen {
			minLen = len(actualSlice)
		}

		for i := 0; i < minLen; i++ {
			filtered = append(filtered, filterToExpectedKeys(expectedSlice[i], actualSlice[i]))
		}
		return filtered
	}

	// For primitives, return actual as-is
	return actual
}
