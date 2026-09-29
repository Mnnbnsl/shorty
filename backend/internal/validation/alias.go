package validation

import (
	"errors"
	"regexp"
	"strings"
)

var (
	aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)

	reservedWords = map[string]struct{}{
		"api":         {},
		"auth":        {},
		"admin":       {},
		"assets":      {},
		"url":         {},
		"health":      {},
		"static":      {},
		"favicon.ico": {},
		"login":       {},
		"register":    {},
		"dashboard":   {},
		"stats":       {},
		"recent":      {},
		"user":        {},
		"logout":      {},
		"null":        {},
		"undefined":   {},
	}

	ErrAliasInvalidChar = errors.New("custom alias must be 3-30 characters long and contain only letters, numbers, dashes, and underscores")
	ErrAliasReserved    = errors.New("this alias is a reserved system keyword and cannot be used")
)

// ValidateCustomAlias checks whether a proposed custom alias meets the syntax and safety rules.
func ValidateCustomAlias(alias string) error {
	trimmed := strings.TrimSpace(alias)
	if !aliasRegex.MatchString(trimmed) {
		return ErrAliasInvalidChar
	}

	if _, ok := reservedWords[strings.ToLower(trimmed)]; ok {
		return ErrAliasReserved
	}

	return nil
}
