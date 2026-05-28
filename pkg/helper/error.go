package helper

import "fmt"

type ErrNotFound struct {
	Resource string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.Resource)
}

type ErrBadRequest struct {
	Message string
}

func (e *ErrBadRequest) Error() string {
	return e.Message
}

type ErrConflict struct {
	Message string
}

func (e *ErrConflict) Error() string {
	return e.Message
}

type ErrForbidden struct {
	Message string
}

func (e *ErrForbidden) Error() string {
	return e.Message
}

type ErrUnauthorized struct {
	Message string
}

func (e *ErrUnauthorized) Error() string {
	return e.Message
}

type ErrUnprocessable struct {
	Message string
}

func (e *ErrUnprocessable) Error() string {
	return e.Message
}

func NewNotFound(resource string) error {
	return &ErrNotFound{Resource: resource}
}

func NewBadRequest(message string) error {
	return &ErrBadRequest{Message: message}
}

func NewConflict(message string) error {
	return &ErrConflict{Message: message}
}

func NewForbidden(message string) error {
	return &ErrForbidden{Message: message}
}

func NewUnauthorized(message string) error {
	return &ErrUnauthorized{Message: message}
}

func NewUnprocessable(message string) error {
	return &ErrUnprocessable{Message: message}
}
