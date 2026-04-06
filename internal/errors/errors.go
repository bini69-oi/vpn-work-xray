package errors

import (
	"errors"
	"fmt"
)

type CodeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *CodeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *CodeError) Unwrap() error { return e.Cause }

func (e *CodeError) Is(target error) bool {
	var other *CodeError
	if !errors.As(target, &other) {
		return false
	}
	return other.Code != "" && e.Code == other.Code
}

func New(code string, message string) *CodeError {
	return &CodeError{Code: code, Message: message}
}

func Wrap(code string, message string, cause error) *CodeError {
	return &CodeError{Code: code, Message: message, Cause: cause}
}

func CodeOf(err error) string {
	var e *CodeError
	if errors.As(err, &e) {
		return e.Code
	}
	return "VPN_UNKNOWN_000"
}

func MessageOf(err error) string {
	var e *CodeError
	if errors.As(err, &e) && e.Message != "" {
		return e.Message
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

var (
	ErrUnauthorized = New("VPN_CONFIG_AUTH_002", "unauthorized")
	ErrDB           = New("VPN_DB_001", "database operation failed")
	ErrCoreStart    = New("VPN_CORE_START_001", "xray runtime start failed")
	ErrConfig       = New("VPN_CONFIG_GEN_001", "config generation failed")
)
