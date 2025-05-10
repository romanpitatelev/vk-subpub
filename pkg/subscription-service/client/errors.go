package client

import "errors"

var (
	ErrKeyEmpty  = errors.New("key cannot be empty")
	ErrDataEmpty = errors.New("data cannot be empty")
)
