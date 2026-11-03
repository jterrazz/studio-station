package fail

import (
	"context"
	"errors"
	"testing"
)

type Code int

const (
	CodeOK          Code = 0
	CodeFailure     Code = 1
	CodeUsage       Code = 2
	CodeInterrupted Code = 130
)

type Error struct {
	Message string
	Hint    string
	Code    Code
	Cause   error
}

func (e *Error) Error() string {
	if e.Message == "" && e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func Usage(message, hint string) *Error {
	guard(hint)
	return &Error{Message: message, Hint: hint, Code: CodeUsage}
}

func Failed(message, hint string, cause error) *Error {
	guard(hint)
	return &Error{Message: message, Hint: hint, Code: CodeFailure, Cause: cause}
}

func Silent(code Code) *Error {
	return &Error{Code: code}
}

func Interrupted() *Error {
	return &Error{Code: CodeInterrupted, Cause: context.Canceled}
}

func CodeOf(err error) Code {
	if err == nil {
		return CodeOK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if errors.Is(err, context.Canceled) {
		return CodeInterrupted
	}
	return CodeFailure
}

func guard(hint string) {
	// invariant: every worded error carries its one repair; under test an empty
	// hint is a bug to fail loudly, at run time kit.Translate fills it in.
	if hint == "" && testing.Testing() {
		panic("fail: an error needs a non-empty hint")
	}
}
