package main

import "errors"

var (
	ErrInsufficientBalance = errors.New("Insufficient balance.")
	ErrDatabase            = errors.New("Database error.")
	ErrInvalidAmount       = errors.New("Invalid amount.")
)
