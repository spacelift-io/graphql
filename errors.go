package graphql

import "fmt"

// OptionError represents an error modifiying a request.
type OptionError struct{ Err error }

func (e *OptionError) Error() string { return fmt.Sprintf("request option error: %v", e.Err) }
func (e *OptionError) Unwrap() error { return e.Err }
