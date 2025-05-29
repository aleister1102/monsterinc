package common

import (
	"fmt"
	"reflect"

	"github.com/rs/zerolog"
)

// ConstructorConfig provides a standardized configuration pattern for constructors
type ConstructorConfig struct {
	Name         string
	Logger       zerolog.Logger
	Config       interface{}
	Dependencies map[string]interface{}
	Validators   []Validator
}

// ConstructorResult represents the result of a constructor call
type ConstructorResult struct {
	Instance interface{}
	Error    error
	Warnings []string
}

// ConstructorBuilder helps build instances with standardized patterns
type ConstructorBuilder struct {
	name         string
	logger       zerolog.Logger
	config       interface{}
	dependencies map[string]interface{}
	validators   []Validator
	warnings     []string
}

// NewConstructorBuilder creates a new constructor builder
func NewConstructorBuilder(name string, logger zerolog.Logger) *ConstructorBuilder {
	return &ConstructorBuilder{
		name:         name,
		logger:       logger.With().Str("constructor", name).Logger(),
		dependencies: make(map[string]interface{}),
		validators:   make([]Validator, 0),
		warnings:     make([]string, 0),
	}
}

// WithConfig sets the configuration for the constructor
func (cb *ConstructorBuilder) WithConfig(config interface{}) *ConstructorBuilder {
	cb.config = config
	return cb
}

// WithDependency adds a dependency to the constructor
func (cb *ConstructorBuilder) WithDependency(name string, dependency interface{}) *ConstructorBuilder {
	cb.dependencies[name] = dependency
	return cb
}

// WithValidator adds a validator to the constructor
func (cb *ConstructorBuilder) WithValidator(validator Validator) *ConstructorBuilder {
	cb.validators = append(cb.validators, validator)
	return cb
}

// AddWarning adds a warning message
func (cb *ConstructorBuilder) AddWarning(message string) *ConstructorBuilder {
	cb.warnings = append(cb.warnings, message)
	return cb
}

// Validate runs all validators and returns any validation errors
func (cb *ConstructorBuilder) Validate() error {
	cb.logger.Debug().
		Int("validator_count", len(cb.validators)).
		Msg("Running constructor validation")

	for i, validator := range cb.validators {
		if err := validator.Validate(); err != nil {
			cb.logger.Error().
				Err(err).
				Int("validator_index", i).
				Msg("Constructor validation failed")
			return fmt.Errorf("validation failed at step %d: %w", i, err)
		}
	}

	cb.logger.Debug().Msg("Constructor validation passed")
	return nil
}

// Build executes the constructor with validation
func (cb *ConstructorBuilder) Build(constructorFunc interface{}) ConstructorResult {
	// Validate first
	if err := cb.Validate(); err != nil {
		return ConstructorResult{
			Instance: nil,
			Error:    err,
			Warnings: cb.warnings,
		}
	}

	cb.logger.Info().
		Str("constructor", cb.name).
		Int("dependency_count", len(cb.dependencies)).
		Msg("Building instance")

	// Use reflection to call the constructor function
	funcValue := reflect.ValueOf(constructorFunc)
	if funcValue.Kind() != reflect.Func {
		return ConstructorResult{
			Instance: nil,
			Error:    fmt.Errorf("constructor must be a function"),
			Warnings: cb.warnings,
		}
	}

	// Prepare arguments based on function signature
	funcType := funcValue.Type()
	args := make([]reflect.Value, funcType.NumIn())

	for i := 0; i < funcType.NumIn(); i++ {
		paramType := funcType.In(i)

		// Try to match parameter by type
		var argValue reflect.Value
		found := false

		// Check if it's the config
		if cb.config != nil && reflect.TypeOf(cb.config) == paramType {
			argValue = reflect.ValueOf(cb.config)
			found = true
		}

		// Check if it's the logger
		if !found && paramType == reflect.TypeOf(cb.logger) {
			argValue = reflect.ValueOf(cb.logger)
			found = true
		}

		// Check dependencies
		if !found {
			for _, dep := range cb.dependencies {
				if reflect.TypeOf(dep) == paramType {
					argValue = reflect.ValueOf(dep)
					found = true
					break
				}
			}
		}

		if !found {
			return ConstructorResult{
				Instance: nil,
				Error:    fmt.Errorf("no matching argument found for parameter %d of type %s", i, paramType),
				Warnings: cb.warnings,
			}
		}

		args[i] = argValue
	}

	// Call the constructor
	results := funcValue.Call(args)

	// Handle results
	var instance interface{}
	var err error

	if len(results) > 0 {
		instance = results[0].Interface()
	}

	if len(results) > 1 && !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	if err != nil {
		cb.logger.Error().Err(err).Msg("Constructor failed")
	} else {
		cb.logger.Info().Msg("Constructor succeeded")
	}

	return ConstructorResult{
		Instance: instance,
		Error:    err,
		Warnings: cb.warnings,
	}
}

// StandardConstructorOrder defines the standard parameter order for constructors
type StandardConstructorOrder struct {
	Config       int // Configuration should come first
	Logger       int // Logger should come second
	Dependencies int // Dependencies come after core parameters
}

// GetStandardOrder returns the recommended parameter order
func GetStandardOrder() StandardConstructorOrder {
	return StandardConstructorOrder{
		Config:       0,
		Logger:       1,
		Dependencies: 2,
	}
}

// ConstructorValidator validates constructor parameters
type ConstructorValidator struct {
	logger zerolog.Logger
}

// NewConstructorValidator creates a new constructor validator
func NewConstructorValidator(logger zerolog.Logger) *ConstructorValidator {
	return &ConstructorValidator{
		logger: logger.With().Str("component", "ConstructorValidator").Logger(),
	}
}

// ValidateParameterOrder validates that constructor parameters follow standard order
func (cv *ConstructorValidator) ValidateParameterOrder(funcType reflect.Type) []string {
	warnings := make([]string, 0)

	if funcType.Kind() != reflect.Func {
		warnings = append(warnings, "not a function type")
		return warnings
	}

	numParams := funcType.NumIn()
	if numParams == 0 {
		return warnings
	}

	// Check for common patterns
	hasConfig := false
	hasLogger := false
	configIndex := -1
	loggerIndex := -1

	for i := 0; i < numParams; i++ {
		paramType := funcType.In(i)
		paramName := paramType.String()

		// Check for config parameter (usually contains "Config" in type name)
		if !hasConfig && (paramType.Kind() == reflect.Ptr || paramType.Kind() == reflect.Struct) {
			if contains(paramName, "Config") {
				hasConfig = true
				configIndex = i
			}
		}

		// Check for logger parameter
		if !hasLogger && paramName == "zerolog.Logger" {
			hasLogger = true
			loggerIndex = i
		}
	}

	// Validate order
	if hasConfig && hasLogger {
		if configIndex > loggerIndex {
			warnings = append(warnings, fmt.Sprintf("config parameter at index %d should come before logger at index %d", configIndex, loggerIndex))
		}
	}

	if hasConfig && configIndex > 2 {
		warnings = append(warnings, fmt.Sprintf("config parameter should be among the first parameters, found at index %d", configIndex))
	}

	if hasLogger && loggerIndex > 3 {
		warnings = append(warnings, fmt.Sprintf("logger parameter should be among the first parameters, found at index %d", loggerIndex))
	}

	return warnings
}

// ValidateNilParameters validates that required parameters are not nil
func (cv *ConstructorValidator) ValidateNilParameters(args []interface{}) error {
	for i, arg := range args {
		if arg == nil {
			return fmt.Errorf("parameter at index %d is nil", i)
		}

		// Check for nil pointers
		argValue := reflect.ValueOf(arg)
		if argValue.Kind() == reflect.Ptr && argValue.IsNil() {
			return fmt.Errorf("parameter at index %d is a nil pointer", i)
		}
	}

	return nil
}

// ConstructorRegistry manages constructor functions and their metadata
type ConstructorRegistry struct {
	constructors map[string]ConstructorMetadata
	logger       zerolog.Logger
}

// ConstructorMetadata holds metadata about a constructor
type ConstructorMetadata struct {
	Name         string
	Function     interface{}
	Description  string
	ParameterDoc map[int]string // Parameter index to documentation
	Examples     []string
	Warnings     []string
}

// NewConstructorRegistry creates a new constructor registry
func NewConstructorRegistry(logger zerolog.Logger) *ConstructorRegistry {
	return &ConstructorRegistry{
		constructors: make(map[string]ConstructorMetadata),
		logger:       logger.With().Str("component", "ConstructorRegistry").Logger(),
	}
}

// Register registers a constructor with metadata
func (cr *ConstructorRegistry) Register(metadata ConstructorMetadata) error {
	if metadata.Name == "" {
		return fmt.Errorf("constructor name cannot be empty")
	}

	if metadata.Function == nil {
		return fmt.Errorf("constructor function cannot be nil")
	}

	// Validate function signature
	funcType := reflect.TypeOf(metadata.Function)
	if funcType.Kind() != reflect.Func {
		return fmt.Errorf("constructor must be a function")
	}

	// Validate parameter order
	validator := NewConstructorValidator(cr.logger)
	warnings := validator.ValidateParameterOrder(funcType)
	metadata.Warnings = append(metadata.Warnings, warnings...)

	cr.constructors[metadata.Name] = metadata

	cr.logger.Info().
		Str("constructor_name", metadata.Name).
		Int("parameter_count", funcType.NumIn()).
		Int("warning_count", len(metadata.Warnings)).
		Msg("Registered constructor")

	return nil
}

// Get retrieves a constructor by name
func (cr *ConstructorRegistry) Get(name string) (ConstructorMetadata, bool) {
	metadata, exists := cr.constructors[name]
	return metadata, exists
}

// List returns all registered constructors
func (cr *ConstructorRegistry) List() map[string]ConstructorMetadata {
	result := make(map[string]ConstructorMetadata)
	for k, v := range cr.constructors {
		result[k] = v
	}
	return result
}

// ValidateAll validates all registered constructors
func (cr *ConstructorRegistry) ValidateAll() map[string][]string {
	results := make(map[string][]string)
	validator := NewConstructorValidator(cr.logger)

	for name, metadata := range cr.constructors {
		funcType := reflect.TypeOf(metadata.Function)
		warnings := validator.ValidateParameterOrder(funcType)
		if len(warnings) > 0 {
			results[name] = warnings
		}
	}

	return results
}

// FactoryPattern provides a standardized factory pattern
type FactoryPattern struct {
	name     string
	logger   zerolog.Logger
	registry *ConstructorRegistry
}

// NewFactoryPattern creates a new factory pattern
func NewFactoryPattern(name string, logger zerolog.Logger) *FactoryPattern {
	return &FactoryPattern{
		name:     name,
		logger:   logger.With().Str("factory", name).Logger(),
		registry: NewConstructorRegistry(logger),
	}
}

// Create creates an instance using the factory pattern
func (fp *FactoryPattern) Create(constructorName string, config ConstructorConfig) ConstructorResult {
	metadata, exists := fp.registry.Get(constructorName)
	if !exists {
		return ConstructorResult{
			Instance: nil,
			Error:    fmt.Errorf("constructor %s not found", constructorName),
			Warnings: []string{},
		}
	}

	builder := NewConstructorBuilder(constructorName, fp.logger).
		WithConfig(config.Config)

	// Add dependencies
	for name, dep := range config.Dependencies {
		builder.WithDependency(name, dep)
	}

	// Add validators
	for _, validator := range config.Validators {
		builder.WithValidator(validator)
	}

	return builder.Build(metadata.Function)
}

// RegisterConstructor registers a constructor with the factory
func (fp *FactoryPattern) RegisterConstructor(metadata ConstructorMetadata) error {
	return fp.registry.Register(metadata)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
