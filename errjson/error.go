package errjson

import (
	"net/http"
)

const (
	ErrorNotValidEntity = 422
)

type RespError struct {
	Type   string `json:"type"`
	Status int    `json:"status"`
	Code   string `json:"code"`
	Data   string `json:"data"`
}

func (e RespError) Error() string {
	return e.Data
}

type NotFoundError struct {
	RespError
}

func NewNotFoundError(msg string) NotFoundError {

	e := NotFoundError{
		RespError: RespError{
			Type:   "error",
			Status: http.StatusNotFound,
			Code:   "404 not found",
			Data:   msg,
		},
	}
	return e
}

type ErrForbidden struct {
	RespError
}

func NewErrForbidden(msg string) ErrForbidden {
	e := ErrForbidden{
		RespError: RespError{
			Type:   "error",
			Status: http.StatusForbidden,
			Code:   "Forbidden",
			Data:   msg,
		},
	}
	return e
}

//422错误
type NotValidEntityError struct {
	RespError
}

func NewNotValidEntityError(msg string) NotValidEntityError {
	e := NotValidEntityError{
		RespError: RespError{
			Type:   "error",
			Status: 422,
			Code:   "Unprocessable Entity",
			Data:   msg,
		},
	}
	return e
}

//401
type UnauthorizedError struct {
	RespError
}

func NewUnauthorizedError(msg string) UnauthorizedError {
	e := UnauthorizedError{
		RespError: RespError{
			Type:   "error",
			Status: http.StatusUnauthorized,
			Code:   "Unauthorized",
			Data:   msg,
		},
	}
	return e
}

//500
type InternalServerError struct {
	RespError
}

func NewInternalServerError(msg string) InternalServerError {
	e := InternalServerError{
		RespError: RespError{
			Type:   "error",
			Status: http.StatusInternalServerError,
			Code:   "internel server error",
			Data:   msg,
		},
	}
	return e
}
