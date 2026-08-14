package strtotime

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrEmptyTimeString      = errors.New("empty time string")
	ErrInvalidTimeUnit      = errors.New("invalid time unit")
	ErrInvalidNumber        = errors.New("invalid number")
	ErrExpectedTimeUnit     = errors.New("expected time unit")
	ErrMissingAmount        = errors.New("missing amount")
	ErrMissingDay           = errors.New("expected day after month name")
	ErrInvalidTimeComponent = errors.New("invalid time component")
	ErrInvalidDateComponent = errors.New("invalid date component")
	ErrInvalidDateFormat    = errors.New("invalid date format")
	ErrInvalidTimezone      = errors.New("invalid timezone")
)

// NewInvalidTimeError returns a formatted error for invalid time components
func NewInvalidTimeError(hour, minute, second int) error {
	return fmt.Errorf("%w: %02d:%02d:%02d", ErrInvalidTimeComponent, hour, minute, second)
}

// NewInvalidDateError returns a formatted error for invalid date components
func NewInvalidDateError(year, month, day int) error {
	return fmt.Errorf("%w: %04d-%02d-%02d", ErrInvalidDateComponent, year, month, day)
}

// IsValidDate checks if the date components form a valid date
func IsValidDate(year, month, day int) bool {
	// Basic validation
	if year < 1 || month < 1 || month > 12 || day < 1 {
		return false
	}

	// Month-specific validation
	maxDays := 31
	switch month {
	case 4, 6, 9, 11: // April, June, September, November
		maxDays = 30
	case 2: // February
		if IsLeapYear(year) {
			maxDays = 29
		} else {
			maxDays = 28
		}
	}

	return day <= maxDays
}

// IsValidTime checks if the time components form a valid time
func IsValidTime(hour, minute, second int) bool {
	return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 && second >= 0 && second <= 59
}

// IsLeapYear determines if a year is a leap year
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
