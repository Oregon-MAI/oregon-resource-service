package models

import "errors"

var ErrNotFound = errors.New("resource not found")

var ErrInvalidStatus = errors.New("invalid resource status")

var ErrInvalidType = errors.New("invalid resource type")
