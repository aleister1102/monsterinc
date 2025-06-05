package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
)

// ConfigValidator handles configuration validation
type ConfigValidator struct {
	validator *validator.Validate
	logger    zerolog.Logger
}

// NewConfigValidator creates a new ConfigValidator with registered custom validations
func NewConfigValidator(logger zerolog.Logger) *ConfigValidator {
	cv := &ConfigValidator{
		validator: validator.New(),
		logger:    logger,
	}

	cv.registerCustomValidations()
	return cv
}

// ValidateConfig performs validation on the GlobalConfig structure.
func ValidateConfig(cfg *GlobalConfig) error {
	logger := zerolog.Nop() // Use nop logger for backward compatibility
	validator := NewConfigValidator(logger)
	return validator.Validate(cfg)
}

// Validate performs validation on the GlobalConfig structure
func (cv *ConfigValidator) Validate(cfg *GlobalConfig) error {
	validationView := cv.createValidationView(cfg)

	if err := cv.validator.Struct(validationView); err != nil {
		return cv.handleValidationError(err)
	}

	cv.logger.Debug().Msg("Configuration validation completed successfully")
	return nil
}

// registerCustomValidations registers all custom validation rules
func (cv *ConfigValidator) registerCustomValidations() {
	cv.registerFileValidations()
	cv.registerURLValidations()
	cv.registerLogValidations()
	cv.registerModeValidations()
	cv.registerSchedulerValidations()
}

// registerFileValidations registers file-related custom validations
func (cv *ConfigValidator) registerFileValidations() {
	// File existence validation
	cv.validator.RegisterValidation("fileexists", func(fl validator.FieldLevel) bool {
		return cv.validateFileExists(fl.Field().String())
	})

	// Directory path validation
	cv.validator.RegisterValidation("dirpath", func(fl validator.FieldLevel) bool {
		return cv.validateDirectoryPath(fl.Field().String())
	})

	// File path format validation
	cv.validator.RegisterValidation("filepath", func(fl validator.FieldLevel) bool {
		return cv.validateFilePath(fl.Field().String())
	})
}

// registerURLValidations registers URL-related custom validations
func (cv *ConfigValidator) registerURLValidations() {
	cv.validator.RegisterValidation("urls", func(fl validator.FieldLevel) bool {
		return cv.validateURLSlice(fl.Field())
	})
}

// registerLogValidations registers logging-related custom validations
func (cv *ConfigValidator) registerLogValidations() {
	cv.validator.RegisterValidation("loglevel", func(fl validator.FieldLevel) bool {
		return cv.validateLogLevel(fl.Field().String())
	})

	cv.validator.RegisterValidation("logformat", func(fl validator.FieldLevel) bool {
		return cv.validateLogFormat(fl.Field().String())
	})
}

// registerModeValidations registers mode-related custom validations
func (cv *ConfigValidator) registerModeValidations() {
	cv.validator.RegisterValidation("mode", func(fl validator.FieldLevel) bool {
		return cv.validateMode(fl.Field().String())
	})
}

// registerSchedulerValidations registers scheduler-related custom validations
func (cv *ConfigValidator) registerSchedulerValidations() {
	cv.validator.RegisterValidation("scanintervaldays", func(fl validator.FieldLevel) bool {
		return fl.Field().Int() >= 1
	})

	cv.validator.RegisterValidation("retryattempts", func(fl validator.FieldLevel) bool {
		return fl.Field().Int() >= 0
	})

	cv.validator.RegisterValidation("sqlitepath", func(fl validator.FieldLevel) bool {
		return fl.Field().String() != ""
	})
}

// validateFileExists checks if a file exists
func (cv *ConfigValidator) validateFileExists(filePath string) bool {
	if filePath == "" {
		return true // Optional field, valid if empty
	}

	fileManager := common.NewFileManager(cv.logger)
	return fileManager.FileExists(filePath)
}

// validateDirectoryPath checks if a directory path exists
func (cv *ConfigValidator) validateDirectoryPath(dirPath string) bool {
	if dirPath == "" {
		return true // Optional field
	}

	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}

// validateFilePath validates file path format
func (cv *ConfigValidator) validateFilePath(filePath string) bool {
	// Basic non-empty check for now, can be enhanced for path validity
	return filePath != ""
}

// validateURLSlice validates a slice of URLs
func (cv *ConfigValidator) validateURLSlice(field reflect.Value) bool {
	if field.Kind() != reflect.Slice {
		return false
	}

	for i := 0; i < field.Len(); i++ {
		urlValue := field.Index(i)
		if urlValue.Kind() != reflect.String {
			continue
		}

		urlStr := urlValue.String()
		if urlStr == "" {
			continue
		}

		// Use urlhandler for consistent URL validation
		if err := urlhandler.ValidateURLFormat(urlStr); err != nil {
			cv.logger.Debug().
				Str("url", urlStr).
				Int("index", i).
				Err(err).
				Msg("Invalid URL in slice")
			return false
		}
	}

	return true
}

// validateLogLevel validates log level values
func (cv *ConfigValidator) validateLogLevel(level string) bool {
	normalizedLevel := strings.ToLower(level)
	validLevels := []string{"", "debug", "info", "warn", "error", "fatal", "panic"}

	for _, validLevel := range validLevels {
		if normalizedLevel == validLevel {
			return true
		}
	}
	return false
}

// validateLogFormat validates log format values
func (cv *ConfigValidator) validateLogFormat(format string) bool {
	normalizedFormat := strings.ToLower(format)
	validFormats := []string{"", "console", "text", "json"}

	for _, validFormat := range validFormats {
		if normalizedFormat == validFormat {
			return true
		}
	}
	return false
}

// validateMode validates mode values
func (cv *ConfigValidator) validateMode(mode string) bool {
	normalizedMode := strings.ToLower(mode)
	validModes := []string{"", "onetime", "automated"}

	for _, validMode := range validModes {
		if normalizedMode == validMode {
			return true
		}
	}
	return false
}

// createValidationView creates a validation view struct for the config
func (cv *ConfigValidator) createValidationView(cfg *GlobalConfig) interface{} {
	return struct {
		PreviousScanLookbackDays int      `validate:"min=1"`
		JSFileExtensions         []string `validate:"dive,required"`
		HTMLFileExtensions       []string `validate:"dive,required"`
		CycleMinutes             int      `validate:"-"`
		RetryAttempts            int      `validate:"-"`
		SQLiteDBPath             string   `validate:"-"`
	}{
		PreviousScanLookbackDays: cfg.DiffConfig.PreviousScanLookbackDays,
		JSFileExtensions:         cfg.MonitorConfig.JSFileExtensions,
		HTMLFileExtensions:       cfg.MonitorConfig.HTMLFileExtensions,
		CycleMinutes:             cfg.SchedulerConfig.CycleMinutes,
		RetryAttempts:            cfg.SchedulerConfig.RetryAttempts,
		SQLiteDBPath:             cfg.SchedulerConfig.SQLiteDBPath,
	}
}

// handleValidationError processes validation errors and returns a meaningful error
func (cv *ConfigValidator) handleValidationError(err error) error {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return common.WrapError(err, "configuration validation error")
	}

	errorMessages := cv.formatValidationErrors(validationErrors)
	return common.NewError("configuration validation failed:\n  %s", strings.Join(errorMessages, "\n  "))
}

// formatValidationErrors formats validation errors into readable messages
func (cv *ConfigValidator) formatValidationErrors(errors validator.ValidationErrors) []string {
	var messages []string

	for _, err := range errors {
		fieldName := cv.getFieldName(err)
		message := cv.formatSingleValidationError(err, fieldName)
		messages = append(messages, message)
	}

	return messages
}

// getFieldName extracts a readable field name from validation error
func (cv *ConfigValidator) getFieldName(err validator.FieldError) string {
	fieldName := err.StructNamespace()

	if strings.Contains(fieldName, ".") {
		parts := strings.Split(fieldName, ".")
		// Find the field name that matches the error field
		for i := len(parts) - 1; i >= 0; i-- {
			if strings.EqualFold(parts[i], err.Field()) {
				fieldName = strings.Join(parts[i:], ".")
				break
			}
		}

		if !strings.HasPrefix(fieldName, err.Field()) {
			fieldName = err.Field()
		}
	}

	return fieldName
}

// formatSingleValidationError formats a single validation error
func (cv *ConfigValidator) formatSingleValidationError(err validator.FieldError, fieldName string) string {
	msg := fmt.Sprintf("Validation failed for '%s': rule '%s'", fieldName, err.Tag())

	if err.Param() != "" {
		msg += fmt.Sprintf(" (expected: %s)", err.Param())
	}

	if err.Value() != nil && err.Value() != "" {
		msg += fmt.Sprintf(", actual: '%v'", err.Value())
	}

	return msg
}
