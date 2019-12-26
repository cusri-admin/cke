package utils

import (
	"fmt"
)

type HTTPError struct {
	Code        int
	Description string
}

func NewHttpError(code int, desc string) *HTTPError {
	return &HTTPError{
		Code:        code,
		Description: desc,
	}
}

func (he *HTTPError) Error() string {
	return fmt.Sprintf("Code: %d, Error: %s", he.Code, he.Description)
}
