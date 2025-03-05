// Package utils provides utility functions used throughout the application.
package utils

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var (
	// scriptTagsRegex matches script tags
	scriptTagsRegex = regexp.MustCompile(`(?i)<script[\s\S]*?>[\s\S]*?</script>`)

	// htmlTagsRegex matches HTML tags
	htmlTagsRegex = regexp.MustCompile(`<[^>]*>`)

	// multipleSpacesRegex matches multiple spaces
	multipleSpacesRegex = regexp.MustCompile(`\s+`)

	// emojiRegex matches common emoji patterns
	emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F700}-\x{1F77F}]|[\x{1F780}-\x{1F7FF}]|[\x{1F800}-\x{1F8FF}]|[\x{1F900}-\x{1F9FF}]|[\x{1FA00}-\x{1FA6F}]|[\x{1FA70}-\x{1FAFF}]|[\x{2600}-\x{26FF}]`)
)

// SanitizeString removes HTML tags and normalizes whitespace
func SanitizeString(s string) string {
	// Remove script tags first for security
	s = scriptTagsRegex.ReplaceAllString(s, "")

	// Remove HTML tags
	s = htmlTagsRegex.ReplaceAllString(s, "")

	// Normalize whitespace
	s = multipleSpacesRegex.ReplaceAllString(s, " ")

	// Trim spaces
	return strings.TrimSpace(s)
}

// SanitizeUsername ensures a username is valid
func SanitizeUsername(username string) string {
	// Trim whitespace
	username = strings.TrimSpace(username)

	// Replace spaces with underscores
	username = strings.ReplaceAll(username, " ", "_")

	// Remove special characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	username = reg.ReplaceAllString(username, "")

	// Ensure it's not too long
	if len(username) > 30 {
		username = username[:30]
	}

	return username
}

// SanitizeEmail ensures an email address format is valid
func SanitizeEmail(email string) string {
	// Trim whitespace
	email = strings.TrimSpace(email)

	// Convert to lowercase
	email = strings.ToLower(email)

	return email
}

// SanitizeURL ensures a URL is valid and safe
func SanitizeURL(rawURL string) string {
	// Trim whitespace
	rawURL = strings.TrimSpace(rawURL)

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Ensure scheme is http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ""
	}

	// Return the normalized URL
	return parsedURL.String()
}

// SanitizeRoomName ensures a room name is valid
func SanitizeRoomName(name string) string {
	// Trim whitespace
	name = strings.TrimSpace(name)

	// Remove excessive whitespace
	name = multipleSpacesRegex.ReplaceAllString(name, " ")

	// Limit length
	if len(name) > 50 {
		name = name[:50]
	}

	return name
}

// StripEmoji removes emoji characters from a string
func StripEmoji(s string) string {
	return emojiRegex.ReplaceAllString(s, "")
}

// CountWords counts the number of words in a string
func CountWords(s string) int {
	s = SanitizeString(s)
	if s == "" {
		return 0
	}

	return len(strings.Fields(s))
}

// LimitWords limits a string to a maximum number of words
func LimitWords(s string, maxWords int) string {
	words := strings.Fields(s)
	if len(words) <= maxWords {
		return s
	}

	return strings.Join(words[:maxWords], " ") + "..."
}

// EscapeHTML escapes HTML special characters
func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")

	return s
}

// StripNonPrintable removes non-printable characters from a string
func StripNonPrintable(s string) string {
	result := strings.Builder{}
	for _, r := range s {
		if unicode.IsPrint(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// SanitizeSearchQuery sanitizes a search query
func SanitizeSearchQuery(query string) string {
	// Trim whitespace
	query = strings.TrimSpace(query)

	// Remove special characters
	reg := regexp.MustCompile(`[^\w\s]`)
	query = reg.ReplaceAllString(query, " ")

	// Normalize whitespace
	query = multipleSpacesRegex.ReplaceAllString(query, " ")

	return query
}

// SanitizeTagList sanitizes a comma-separated list of tags
func SanitizeTagList(tags string) []string {
	// Split by comma
	tagList := strings.Split(tags, ",")

	result := make([]string, 0, len(tagList))
	for _, tag := range tagList {
		// Trim whitespace
		tag = strings.TrimSpace(tag)

		// Skip empty tags
		if tag == "" {
			continue
		}

		// Sanitize each tag
		tag = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(tag, "")

		// Limit length
		if len(tag) > 20 {
			tag = tag[:20]
		}

		// Add to result if not empty
		if tag != "" {
			result = append(result, tag)
		}
	}

	return result
}
