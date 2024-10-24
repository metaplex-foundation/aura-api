package errors

import (
	"errors"
	"strings"

	"firebase.google.com/go/v4/errorutils"
)

var (
	ErrNeedEmailVerification   = "need email verification"
	ErrProjectNotFound         = "project not found"
	ErrUserNotFound            = errors.New("USER_NOT_FOUND")
	ErrNotFound                = errors.New("NOT_FOUND")
	ErrAlreadyExists           = errors.New("ALREADY_EXISTS")
	ErrInvalidArgument         = errors.New("INVALID_ARGUMENT")
	ErrTooManyAttemptsTryLater = errors.New("TOO_MANY_ATTEMPTS_TRY_LATER")
)

func HandleFirebaseError(err error) error {
	switch {
	case strings.Contains(err.Error(), ErrTooManyAttemptsTryLater.Error()):
		return ErrTooManyAttemptsTryLater
	case errorutils.IsNotFound(err):
		return ErrNotFound
	case errorutils.IsAlreadyExists(err):
		return ErrAlreadyExists
	case errorutils.IsInvalidArgument(err):
		return ErrInvalidArgument
	}

	return nil
}
