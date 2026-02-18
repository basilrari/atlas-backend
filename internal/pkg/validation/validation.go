package validation

import (
	"regexp"
	"unicode"
)

// isValidEmail matches Express: /^[^\s@]+@[^\s@]+\.[^\s@]+$/
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Fullname: letters, spaces, hyphens, apostrophes only (Express nameRegex).
var fullnameRe = regexp.MustCompile(`^[A-Za-z\s\-']+$`)

func IsValidEmail(email string) bool {
	return emailRe.MatchString(email)
}

// IsValidPassword enforces the same rule as Express utils/validation.js:
// - at least 8 characters
// - contains at least one letter
// - contains at least one number
// - contains at least one special character
func IsValidPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLetter, hasDigit, hasSpecial := false, false, false
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	return hasLetter && hasDigit && hasSpecial
}

func IsValidFullname(fullname string) bool {
	return fullname != "" && fullnameRe.MatchString(fullname)
}
