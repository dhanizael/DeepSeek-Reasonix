package builtin

import (
	"fmt"
	"sync/atomic"

	"reasonix/internal/repair"
)

// Package-level AST guard: default on so Go/JSON agent writes are validated
// before they hit disk/overlay. Boot may flip this from [enhanced.ast_guard].
var astSyntaxGuardEnabled atomic.Bool

func init() {
	astSyntaxGuardEnabled.Store(true)
}

var defaultASTSyntaxGuard = repair.NewASTSyntaxGuard()

// SetASTSyntaxGuardEnabled toggles pre-write syntax validation for write_file,
// edit_file, and multi_edit. Safe for concurrent use.
func SetASTSyntaxGuardEnabled(enabled bool) {
	astSyntaxGuardEnabled.Store(enabled)
}

// ASTSyntaxGuardEnabled reports whether pre-write syntax validation is active.
func ASTSyntaxGuardEnabled() bool {
	return astSyntaxGuardEnabled.Load()
}

// rejectInvalidSyntax returns a non-nil error when path's extension is checked
// and content fails validation. Unsupported extensions pass through.
func rejectInvalidSyntax(path, content string) error {
	if !astSyntaxGuardEnabled.Load() {
		return nil
	}
	ok, detail := defaultASTSyntaxGuard.ValidateSyntax(path, content)
	if ok {
		return nil
	}
	if detail == "" {
		return fmt.Errorf("syntax validation rejected write to %s", path)
	}
	return fmt.Errorf("%s", detail)
}
