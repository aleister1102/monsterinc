package config

import (
	"errors"
	"fmt"
	"monsterinc/internal/urlhandler"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ValidateConfig performs validation on the GlobalConfig structure.
func ValidateConfig(cfg *GlobalConfig) error {
	validate := validator.New()

	// Register custom validation for file existence
	_ = validate.RegisterValidation("fileexists", func(fl validator.FieldLevel) bool {
		filePath := fl.Field().String()
		if filePath == "" {
			return true // Optional field, valid if empty
		}
		_, err := os.Stat(filePath)
		return !os.IsNotExist(err) // True if file exists or other error (e.g. permission denied)
	})

	// Register custom validation for directory path existence (basic check)
	_ = validate.RegisterValidation("dirpath", func(fl validator.FieldLevel) bool {
		dirPath := fl.Field().String()
		if dirPath == "" {
			return true // Optional field
		}
		info, err := os.Stat(dirPath)
		if os.IsNotExist(err) {
			return false
		}
		return err == nil && info.IsDir()
	})

	// Register custom validation for general file path (does not check existence, just format - can be extended)
	_ = validate.RegisterValidation("filepath", func(fl validator.FieldLevel) bool {
		// For now, a basic non-empty check. Can be enhanced for path validity.
		return fl.Field().String() != "" || !fl.Parent().FieldByName(fl.FieldName()).IsValid() // Valid if field is not empty or is optional and not set
	})

	// Register custom validation for slices of URLs (ensure they are valid URLs)
	_ = validate.RegisterValidation("urls", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.Slice {
			return false
		}
		slice, ok := fl.Field().Interface().([]string)
		if !ok {
			return false // Should not happen if struct tag is on a []string
		}
		for _, s := range slice {
			if err := urlhandler.ValidateURLFormat(s); err != nil {
				return false
			}
		}
		return true
	})

	// Register custom validation for LogLevel
	_ = validate.RegisterValidation("loglevel", func(fl validator.FieldLevel) bool {
		level := strings.ToLower(fl.Field().String())
		switch level {
		case "", "debug", "info", "warn", "error", "fatal", "panic": // Allow empty for omitempty
			return true
		default:
			return false
		}
	})

	// Register custom validation for LogFormat
	_ = validate.RegisterValidation("logformat", func(fl validator.FieldLevel) bool {
		format := strings.ToLower(fl.Field().String())
		switch format {
		case "", "console", "text", "json": // Allow empty for omitempty
			return true
		default:
			return false
		}
	})

	// Register custom validation for Mode
	_ = validate.RegisterValidation("mode", func(fl validator.FieldLevel) bool {
		mode := strings.ToLower(fl.Field().String())
		switch mode {
		case "", "onetime", "automated": // Allow empty for omitempty, or specific values
			return true
		default:
			return false
		}
	})

	// Register custom validation for SchedulerConfig fields
	_ = validate.RegisterValidation("scanintervaldays", func(fl validator.FieldLevel) bool {
		days := fl.Field().Int()
		return days >= 1
	})
	_ = validate.RegisterValidation("retryattempts", func(fl validator.FieldLevel) bool {
		attempts := fl.Field().Int()
		return attempts >= 0
	})
	_ = validate.RegisterValidation("sqlitepath", func(fl validator.FieldLevel) bool {
		path := fl.Field().String()
		return path != ""
	})

	validationView := struct {
		PreviousScanLookbackDays int      `validate:"min=1"`
		JSFileExtensions         []string `validate:"dive,required"`
		HTMLFileExtensions       []string `validate:"dive,required"`
		// SchedulerConfig fields - struct tags in SchedulerConfig itself handle validation rules.
		// These are here to ensure they appear in user-friendly error messages if validation fails.
		CycleMinutes  int    `validate:"-"` // Validation handled by tag in SchedulerConfig
		RetryAttempts int    `validate:"-"` // Validation handled by tag in SchedulerConfig
		SQLiteDBPath  string `validate:"-"` // Validation handled by tag in SchedulerConfig
	}{
		PreviousScanLookbackDays: cfg.DiffConfig.PreviousScanLookbackDays,
		JSFileExtensions:         cfg.MonitorConfig.JSFileExtensions,
		HTMLFileExtensions:       cfg.MonitorConfig.HTMLFileExtensions,
		// Add SchedulerConfig fields for validation error reporting
		CycleMinutes:  cfg.SchedulerConfig.CycleMinutes,
		RetryAttempts: cfg.SchedulerConfig.RetryAttempts,
		SQLiteDBPath:  cfg.SchedulerConfig.SQLiteDBPath,
	}

	err := validate.Struct(validationView)
	if err != nil {
		var errs validator.ValidationErrors
		if errors.As(err, &errs) {
			var validationErrorMessages []string
			for _, e := range errs {
				fieldName := e.StructNamespace()
				// Attempt to get the actual field name if it's nested
				if strings.Contains(fieldName, ".") {
					parts := strings.Split(fieldName, ".")
					// Heuristic: find the field name that matches e.Field()
					// This might need refinement for complex nested structs
					for i := len(parts) - 1; i >= 0; i-- {
						if strings.EqualFold(parts[i], e.Field()) {
							fieldName = strings.Join(parts[i:], ".")
							break
						}
					}
					if !strings.HasPrefix(fieldName, e.Field()) { // Fallback if heuristic fails
						fieldName = e.Field()
					}
				}
				msg := fmt.Sprintf("Validation failed for '%s': rule '%s'", fieldName, e.Tag())
				if e.Param() != "" {
					msg += fmt.Sprintf(" (expected: %s)", e.Param())
				}
				if e.Value() != nil && e.Value() != "" {
					msg += fmt.Sprintf(", actual: '%v'", e.Value())
				}
				validationErrorMessages = append(validationErrorMessages, msg)
			}
			return fmt.Errorf("configuration validation failed:\n  %s", strings.Join(validationErrorMessages, "\n  "))
		}
		return fmt.Errorf("configuration validation error: %w", err)
	}
	return nil
}
