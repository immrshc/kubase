package errors

import (
	"fmt"
	"github.com/pkg/errors"
	"os"
)

const (
	StandardError ErrorType = iota
	ArgumentError
	FileOperationError
	UnmarshalInvalidFormatError
)

type ErrorType int

type customError struct {
	errorType ErrorType
	error     error
}

func (ce customError) Error() string {
	return ce.error.Error()
}

func (ce customError) Type() ErrorType {
	return ce.errorType
}

func (ec ErrorType) New(msg string) error {
	return customError{errorType: ec, error: errors.New(msg)}
}

func (ec ErrorType) Newf(msg string, args ...interface{}) error {
	return customError{errorType: ec, error: errors.New(fmt.Sprintf(msg, args...))}
}

func (ec ErrorType) Wrap(err error, msg string) error {
	return customError{errorType: ec, error: errors.Wrap(err, msg)}
}

func (ec ErrorType) Wrapf(err error, msg string, args ...interface{}) error {
	return customError{errorType: ec, error: errors.Wrapf(err, msg, args...)}
}

func New(msg string) error {
	return customError{errorType: StandardError, error: errors.New(msg)}
}

func Newf(msg string, args ...interface{}) error {
	return customError{errorType: StandardError, error: errors.New(fmt.Sprintf(msg, args...))}
}

func Wrap(err error, msg string) error {
	return Wrapf(err, msg)
}

func Wrapf(err error, msg string, args ...interface{}) error {
	wrappedError := errors.Wrapf(err, msg, args...)
	if customErr, ok := err.(customError); ok {
		return customError{
			errorType: customErr.errorType,
			error:     wrappedError,
		}
	}
	return customError{errorType: StandardError, error: wrappedError}
}

func Cause(err error) error {
	return errors.Cause(err)
}

func GetType(err error) ErrorType {
	if ce, ok := Cause(err).(customError); ok {
		return ce.Type()
	}
	return StandardError
}

func CheckError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR %v\n", err)
	}
}
