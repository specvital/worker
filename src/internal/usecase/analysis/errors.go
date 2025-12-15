package analysis

import "errors"

var (
	// ErrCloneFailed indicates VCS clone operation failed.
	ErrCloneFailed = errors.New("clone failed")

	// ErrScanFailed indicates parser scan operation failed.
	ErrScanFailed = errors.New("scan failed")

	// ErrSaveFailed indicates repository save operation failed.
	ErrSaveFailed = errors.New("save failed")
)
