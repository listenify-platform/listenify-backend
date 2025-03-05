// Package utils provides utility functions used throughout the application.
package utils

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Define a custom type for context keys to avoid collisions
type contextKey string

// RequestContextKey is used to store and retrieve the HTTP request in a context
const RequestContextKey contextKey = "request"

// GenerateRandomBytes generates n random bytes
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString generates a random string of length n
func GenerateRandomString(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:n], nil
}

// GenerateRandomHex generates a random hex string of length n
func GenerateRandomHex(n int) (string, error) {
	bytes, err := GenerateRandomBytes(n / 2)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateID generates a unique ID with a specified prefix
func GenerateID(prefix string) (string, error) {
	randomPart, err := GenerateRandomHex(16)
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Unix()

	if prefix == "" {
		return fmt.Sprintf("%x%s", timestamp, randomPart), nil
	}

	return fmt.Sprintf("%s_%x%s", prefix, timestamp, randomPart), nil
}

// ParseInt parses a string into an int64 with a default value on error
func ParseInt(s string, defaultValue int64) int64 {
	if s == "" {
		return defaultValue
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultValue
	}

	return val
}

// ParseBool parses a string into a boolean with a default value on error
func ParseBool(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}

	val, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}

	return val
}

// ParseFloat parses a string into a float64 with a default value on error
func ParseFloat(s string, defaultValue float64) float64 {
	if s == "" {
		return defaultValue
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultValue
	}

	return val
}

// TruncateString truncates a string to the specified max length with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}

var youtubeRegex = regexp.MustCompile(`(?:youtube\.com\/(?:[^\/\n\s]+\/\S+\/|(?:v|e(?:mbed)?)\/|\S*?[?&]v=)|youtu\.be\/)([a-zA-Z0-9_-]{11})`)

// ExtractYouTubeID extracts the video ID from a YouTube URL
func ExtractYouTubeID(url string) string {
	// Try to match youtube.com URLs
	matches := youtubeRegex.FindStringSubmatch(url)

	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// ExtractSoundCloudURL normalizes a SoundCloud URL
func ExtractSoundCloudURL(url string) string {
	// Check if URL contains soundcloud.com
	if !strings.Contains(url, "soundcloud.com") {
		return ""
	}

	// Remove query parameters
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// Ensure URL starts with https://
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = "https://" + url
	}

	return url
}

// FormatDuration formats seconds into a human-readable duration (MM:SS)
func FormatDuration(seconds int) string {
	minutes := seconds / 60
	remainingSeconds := seconds % 60

	return fmt.Sprintf("%02d:%02d", minutes, remainingSeconds)
}

// FormatDurationLong formats seconds into a longer human-readable format (HH:MM:SS)
func FormatDurationLong(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainingSeconds := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, remainingSeconds)
	}

	return fmt.Sprintf("%02d:%02d", minutes, remainingSeconds)
}

// HumanizeTime formats a time.Time into a human-readable relative time
func HumanizeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		minutes := int(math.Floor(diff.Minutes()))
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case diff < time.Hour*24:
		hours := int(math.Floor(diff.Hours()))
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < time.Hour*24*7:
		days := int(math.Floor(diff.Hours() / 24))
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < time.Hour*24*30:
		weeks := int(math.Floor(diff.Hours() / 24 / 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < time.Hour*24*365:
		months := int(math.Floor(diff.Hours() / 24 / 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(math.Floor(diff.Hours() / 24 / 365))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// ParseTimestamp parses a unix timestamp string into a time.Time
func ParseTimestamp(timestamp string) (time.Time, error) {
	i, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(i, 0), nil
}

// JSONResponse writes a JSON response with the given status code and data
func JSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// If encoding fails, log error and send simple error message
			GetLogger().Error("Failed to encode JSON response", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"Internal server error"}`))
		}
	}
}

// ErrorJSON writes a JSON error response with the given status code and error
func ErrorJSON(w http.ResponseWriter, statusCode int, err error) {
	response := ErrorResponse(err)
	JSONResponse(w, statusCode, response)
}

// GetRequestIP gets the client IP address from the request
func GetRequestIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		// If no proxy, get the remote address
		ip = r.RemoteAddr
	}

	// If there are multiple IPs in X-Forwarded-For, get the first one
	if strings.Contains(ip, ",") {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	}

	// Remove port number if present
	if strings.Contains(ip, ":") {
		ip = strings.Split(ip, ":")[0]
	}

	return ip
}

// Retry executes the given function with retries
func Retry(attempts int, sleep time.Duration, fn func() error) error {
	var err error

	for i := range attempts {
		err = fn()
		if err == nil {
			return nil
		}

		// Sleep before retrying, with exponential backoff
		if i < attempts-1 {
			sleepTime := sleep * time.Duration(math.Pow(2, float64(i)))
			time.Sleep(sleepTime)
		}
	}

	return err
}

// Contains checks if a string slice contains a specific string
//
// Deprecated: Use slices.Contains instead
func Contains(slice []string, str string) bool {
	return slices.Contains(slice, str)
}

// Map applies a function to each element of a slice and returns a new slice
//
// Deprecated: Use provided method from github.com/samber/lo package instead
func Map[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i, t := range ts {
		us[i] = f(t)
	}
	return us
}

// Filter returns a new slice containing only the elements that satisfy the predicate
//
// Deprecated: Use provided method from github.com/samber/lo package instead
func Filter[T any](ts []T, f func(T) bool) []T {
	var result []T
	for _, t := range ts {
		if f(t) {
			result = append(result, t)
		}
	}
	return result
}

// GetPageParams extracts pagination parameters from an HTTP request
func GetPageParams(r *http.Request, defaultLimit int) (page, limit int) {
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page = max(int(ParseInt(pageStr, 1)), 1)

	limit = int(ParseInt(limitStr, int64(defaultLimit)))
	if limit < 1 || limit > 100 {
		limit = defaultLimit
	}

	return page, limit
}

// SlugifyString creates a URL-friendly slug from a string
func SlugifyString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Remove special characters
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")

	// Convert spaces to hyphens
	s = regexp.MustCompile(`[\s]+`).ReplaceAllString(s, "-")

	// Remove consecutive hyphens
	s = regexp.MustCompile(`[-]+`).ReplaceAllString(s, "-")

	// Remove leading and trailing hyphens
	s = strings.Trim(s, "-")

	return s
}

func HasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func HasLowercase(s string) bool {
	for _, r := range s {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

func HasNumber(s string) bool {
	for _, char := range s {
		if unicode.IsDigit(char) {
			return true
		}
	}
	return false
}

func HasSpecialChar(s string) bool {
	for _, char := range s {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) {
			return true
		}
	}
	return false
}

func ParseRedisInfo(info string) map[string]string {
	parsedInfo := make(map[string]string)
	lines := strings.SplitSeq(info, "\n")
	for line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				parsedInfo[parts[0]] = parts[1]
			}
		}
	}
	return parsedInfo
}

func SortSlice[T any](slice []T, less func(i, j int) bool) {
	sort.Slice(slice, less)
}

func SplitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}

func GetRequestIPFromContext(ctx context.Context) string {
	r := ctx.Value(RequestContextKey).(*http.Request)
	return GetRequestIP(r)
}

func GetRequestUserAgentFromContext(ctx context.Context) string {
	r := ctx.Value(RequestContextKey).(*http.Request)
	return r.UserAgent()
}
