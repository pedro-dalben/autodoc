package cli

import (
	"errors"
	"fmt"
	"strings"
)

// CodedError is a compact agent-facing error. Cause remains available through
// errors.Unwrap; the concise code avoids sending a browser stack trace when a
// deterministic recovery step is enough.
type CodedError struct {
	Code  string
	Loc   string
	Cause error
	Hint  string
}

func (e *CodedError) Error() string {
	where := ""
	if e.Loc != "" {
		where = " " + e.Loc
	}
	return fmt.Sprintf("%s%s: %v", e.Code, where, e.Cause)
}
func (e *CodedError) Unwrap() error { return e.Cause }

// FormatError translates known operational failures; unknown failures retain
// their original text instead of being mislabeled.
func FormatError(err error) string {
	var coded *CodedError
	if errors.As(err, &coded) {
		return formatCoded(coded)
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "target") && (strings.Contains(lower, "not found") || strings.Contains(lower, "requires target")):
		return formatCoded(&CodedError{Code: "E_TARGET_MISSING", Cause: err, Hint: "run autodoc ui query, then use a stable target"})
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return formatCoded(&CodedError{Code: "E_WAIT_TIMEOUT", Cause: err, Hint: "inspect the state delta or increase the explicit wait"})
	case strings.Contains(lower, "storyboard") && strings.Contains(lower, "invalid"):
		return formatCoded(&CodedError{Code: "E_STORYBOARD_INVALID", Cause: err, Hint: "fix the named field and rerun storyboard validate"})
	case strings.Contains(lower, "auth") || strings.Contains(lower, "storage state"):
		return formatCoded(&CodedError{Code: "E_AUTH_STALE", Cause: err, Hint: "refresh the dedicated AutoDoc auth state"})
	default:
		return "FAIL: " + err.Error()
	}
}

func formatCoded(e *CodedError) string {
	where := ""
	if e.Loc != "" {
		where = " " + e.Loc
	}
	hint := ""
	if e.Hint != "" {
		hint = "\nhint: " + e.Hint
	}
	return fmt.Sprintf("FAIL [%s]%s\ncause: %v%s", e.Code, where, e.Cause, hint)
}
