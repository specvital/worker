package service

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrCloneFailed  = errors.New("clone failed")
	ErrScanFailed   = errors.New("scan failed")
	ErrSaveFailed   = errors.New("save failed")
)
