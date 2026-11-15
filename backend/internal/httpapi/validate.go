package httpapi

import (
	"slices"
	"strings"
	"unicode/utf8"
)

const maxNameLength = 40

// relationships are the allowed values of members.relationship.
var relationships = []string{"Mom", "Dad", "Grandma", "Grandpa", "Other"}

// cleanName trims spaces and checks the length is 1 to 40 characters.
func cleanName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= 1 && n <= maxNameLength
}

// validateMember checks a member's name and relationship. It returns the
// trimmed name, or a message for the user when the input is invalid.
func validateMember(name, relationship string) (cleaned, problem string) {
	cleaned, ok := cleanName(name)
	if !ok {
		return "", "Your name must be 1 to 40 characters."
	}
	if !slices.Contains(relationships, relationship) {
		return "", "Relationship must be one of: " + strings.Join(relationships, ", ") + "."
	}
	return cleaned, ""
}
