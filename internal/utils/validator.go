// Package utils provides utility functions used throughout the application.
package utils

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	// validate is a singleton validator instance
	validate *validator.Validate

	// usernameRegex defines valid username characters (letters, numbers, underscores, hyphens)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// Custom error messages for validation errors
	validationErrorMessages = map[string]string{
		"required":       "This field is required",
		"email":          "Invalid email address",
		"min":            "Value must be greater than or equal to %s",
		"max":            "Value must be less than or equal to %s",
		"len":            "Length must be exactly %s",
		"alphanum":       "Must contain only alphanumeric characters",
		"username":       "Username must contain only letters, numbers, underscores or hyphens",
		"password":       "Password must be at least 8 characters and contain uppercase, lowercase, and numbers",
		"url":            "Must be a valid URL",
		"youtube_url":    "Must be a valid YouTube URL",
		"soundcloud_url": "Must be a valid SoundCloud URL",
	}
)

// Initialize validator with custom validations
func init() {
	validate = validator.New()

	// Register function to get tag name from json tags
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Register custom validation functions
	_ = validate.RegisterValidation("username", validateUsername)
	_ = validate.RegisterValidation("password", validatePassword)
	_ = validate.RegisterValidation("youtube_url", validateYouTubeURL)
	_ = validate.RegisterValidation("soundcloud_url", validateSoundCloudURL)
}

// Validate performs validation on the given struct and returns validation errors.
func Validate(s any) error {
	return validate.Struct(s)
}

// ValidateVar validates a single variable with the given tag and returns errors.
func ValidateVar(field any, tag string) error {
	return validate.Var(field, tag)
}

// FormatValidationErrors formats validation errors into a user-friendly map.
func FormatValidationErrors(err error) map[string]string {
	if err == nil {
		return nil
	}

	validationErrors := make(map[string]string)
	for _, err := range err.(validator.ValidationErrors) {
		field := err.Field()
		tag := err.Tag()
		param := err.Param()

		message, exists := validationErrorMessages[tag]
		if !exists {
			message = "Invalid value"
		}

		// Replace parameter placeholders in error messages
		if param != "" && strings.Contains(message, "%s") {
			message = strings.Replace(message, "%s", param, 1)
		}

		validationErrors[field] = message
	}

	return validationErrors
}

// Custom validation functions

// validateUsername checks if a string is a valid username.
func validateUsername(fl validator.FieldLevel) bool {
	return usernameRegex.MatchString(fl.Field().String())
}

// validatePassword checks if a string is a valid password.
// Password must be at least 8 characters and contain uppercase, lowercase, and numbers.
func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	// Length check
	if len(password) < 8 {
		return false
	}

	// Character type checks
	var (
		hasUpper  bool
		hasLower  bool
		hasNumber bool
	)

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		}
	}

	return hasUpper && hasLower && hasNumber
}

// Regular expressions for URL validation
var (
	youtubeURLRegex    = regexp.MustCompile(`^(https?://)?(www\.)?(youtube\.com|youtu\.be)/.+$`)
	soundcloudURLRegex = regexp.MustCompile(`^(https?://)?(www\.)?soundcloud\.com/.+$`)
)

// validateYouTubeURL checks if a string is a valid YouTube URL.
func validateYouTubeURL(fl validator.FieldLevel) bool {
	return youtubeURLRegex.MatchString(fl.Field().String())
}

// validateSoundCloudURL checks if a string is a valid SoundCloud URL.
func validateSoundCloudURL(fl validator.FieldLevel) bool {
	return soundcloudURLRegex.MatchString(fl.Field().String())
}

// ValidationRequest provides a generic validation request structure
type ValidationRequest struct {
	Data any `json:"data"`
}

// ValidationResponse provides a generic validation response structure
type ValidationResponse struct {
	Valid  bool              `json:"valid"`
	Errors map[string]string `json:"errors,omitempty"`
}

// ValidateJSON validates the given JSON request against the validation rules.
func ValidateJSON(data any) ValidationResponse {
	err := Validate(data)
	if err != nil {
		return ValidationResponse{
			Valid:  false,
			Errors: FormatValidationErrors(err),
		}
	}

	return ValidationResponse{
		Valid: true,
	}
}

// GetValidator returns the validator instance.
func GetValidator() *validator.Validate {
	return validate
}
