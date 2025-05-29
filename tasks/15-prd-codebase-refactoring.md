# PRD: Codebase Refactoring

## Introduction/Overview

The MonsterInc project currently has code quality issues that need to be addressed through comprehensive codebase refactoring. The main issues include: code duplication, verbose comments, unused dependencies, excessive logging noise, and inconsistent patterns. The goal of this refactoring is to create a maintainable codebase that is easy to extend with new features and reduce technical debt.

## Goals

1. **Reduce Code Duplication**: Identify and eliminate duplicate code patterns throughout the project
2. **Clean Up Comments**: Shorten verbose comments and keep only necessary ones
3. **Remove Unused Dependencies**: Remove unused dependencies from go.mod
4. **Reduce Logging Noise**: Minimize unnecessary Info/Debug logs
5. **Improve Maintainability**: Create a codebase that is easy to maintain and extend with new features
6. **Maintain Backward Compatibility**: Ensure functionality remains unchanged after each refactoring phase
7. **Increase Common Package Usage**: Maximize reuse of utilities in internal/common across all internal packages
8. **Consolidate URL Operations**: Centralize all URL-related functions into urlhandler package
9. **Organize Shared Models**: Move shared data structures to models package for better organization
10. **Improve Function Readability**: Break down large functions into smaller, focused functions

## User Stories

1. **As a developer**, I want the codebase to have minimal code duplication so I can easily maintain and add new features without copy-pasting logic.

2. **As a developer**, I want concise and to-the-point comments so I can read code without being distracted by verbose explanations.

3. **As a developer**, I want clean dependencies so build time is faster and the dependency tree is simpler.

4. **As a system operator**, I want less noisy logs so I can easily debug and monitor the system.

5. **As a developer**, I want consistent patterns throughout the codebase so it's easy to understand and contribute.

6. **As a developer**, I want standardized context cancellation patterns so I can easily understand and debug concurrent operations.

7. **As a developer**, I want consistent error handling patterns so I can easily trace and handle errors across services.

8. **As a developer**, I want all services to use common HTTP client utilities so configuration is consistent and maintainable.

9. **As a developer**, I want all URL operations centralized in urlhandler so there's a single source of truth for URL processing.

10. **As a developer**, I want smaller, focused functions so I can easily understand and test individual pieces of functionality.

11. **As a developer**, I want consistent Discord message formatting with standardized colors and styles for better user experience.

## Functional Requirements

### FR1: Code Duplication Elimination
1. **FR1.1**: Create shared utility functions for common error handling patterns
2. **FR1.2**: Consolidate constructor patterns (all New* functions accept logger and config)
3. **FR1.3**: Create shared interfaces for common behaviors (Validator, Initializer, etc.)
4. **FR1.4**: Extract common HTTP client configuration logic
5. **FR1.5**: Consolidate file I/O patterns and error handling
6. **FR1.6**: Extract duplicated context cancellation patterns from orchestrator.go, scheduler.go, and service.go
7. **FR1.7**: Consolidate common workflow execution patterns (scan lifecycle, retry logic, etc.)
8. **FR1.8**: Extract shared notification patterns and summary data preparation
9. **FR1.9**: Standardize service initialization patterns (Start/Stop lifecycle)
10. **FR1.10**: Consolidate duplicated logging patterns with consistent message formats

### FR2: Common Package Integration
11. **FR2.1**: Refactor fetcher.go to use internal/common/http_client.go instead of creating HTTP clients manually
12. **FR2.2**: Update all services to use common/file_utils.go for file operations instead of direct os package calls
13. **FR2.3**: Migrate all services to use common/errors.go for error handling and wrapping
14. **FR2.4**: Replace manual HTTP client creation across all packages with common/http_client.go factories
15. **FR2.5**: Update all packages to use common/interfaces.go for consistent behavior contracts
16. **FR2.6**: Ensure all constructors use common error handling and validation patterns
17. **FR2.7**: Replace direct context usage with common context utilities (to be created)

### FR3: URL Function Consolidation
18. **FR3.1**: Move URL validation functions from scheduler/target_manager.go to urlhandler package
19. **FR3.2**: Migrate URL processing functions from various packages to urlhandler.go
20. **FR3.3**: Consolidate URL sanitization functions (like SanitizeFilenameForMonitoring) into urlhandler
21. **FR3.4**: Move URL hostname extraction functions from crawler package to urlhandler
22. **FR3.5**: Centralize all URL normalization and resolution logic in urlhandler package
23. **FR3.6**: Update all packages to import and use urlhandler functions instead of local implementations
24. **FR3.7**: Create comprehensive URL utility functions for common operations (parsing, validation, transformation)

### FR4: Model Organization and Shared Data Structures
25. **FR4.1**: Move shared configuration models from individual packages to models package
26. **FR4.2**: Consolidate duplicate model definitions across packages into models package
27. **FR4.3**: Move common data transfer objects (DTOs) to models package for reuse
28. **FR4.4**: Organize models by domain (scan, monitor, notification, etc.) within models package
29. **FR4.5**: Create shared interfaces for common model behaviors in models package
30. **FR4.6**: Update all packages to use centralized models instead of local definitions
31. **FR4.7**: Ensure model consistency and remove redundant type definitions

### FR5: Function Breakdown for Readability
32. **FR5.1**: Break down ExecuteScanWorkflow in orchestrator.go into smaller, focused functions
33. **FR5.2**: Split large functions in scheduler.go (runScanCycle, runScanCycleWithRetries) into logical components
34. **FR5.3**: Decompose checkURL function in monitor/service.go into smaller processing steps
35. **FR5.4**: Break down large functions in parquet_file_history_store.go for better maintainability
36. **FR5.5**: Split complex functions in secrets_store.go into focused operations
37. **FR5.6**: Decompose ExtractPaths function in path_extractor.go into smaller processing steps
38. **FR5.7**: Ensure each function has a single responsibility and clear purpose

### FR6: Discord Message Formatting Standardization
39. **FR6.1**: Standardize color scheme across all Discord message types (success, error, warning, info)
40. **FR6.2**: Create consistent embed field structures for similar message types
41. **FR6.3**: Standardize timestamp formatting and display across all Discord messages
42. **FR6.4**: Unify footer information and branding across all Discord notifications
43. **FR6.5**: Create consistent field naming and value formatting patterns
44. **FR6.6**: Standardize error message formatting and severity indicators
45. **FR6.7**: Ensure consistent username and avatar usage across all webhook notifications

### FR7: Comment Cleanup
46. **FR7.1**: Remove comments longer than 2 lines that explain obvious code logic
47. **FR7.2**: Keep only comments for complex business logic or external dependencies
48. **FR7.3**: Remove outdated TODO/FIXME comments that are no longer relevant
49. **FR7.4**: Standardize comment format (// TODO: instead of // Task X.X)

### FR8: Dependency Management
50. **FR8.1**: Audit all dependencies in go.mod
51. **FR8.2**: Remove dependencies not imported in any Go file
52. **FR8.3**: Check if indirect dependencies can be removed
53. **FR8.4**: Update go.mod and go.sum after cleanup

### FR9: Logging Optimization
54. **FR9.1**: Audit all log.Info() and log.Debug() calls
55. **FR9.2**: Keep only essential Info logs (startup, shutdown, major operations)
56. **FR9.3**: Convert unnecessary Info logs to Debug level
57. **FR9.4**: Remove redundant debug logs in hot paths
58. **FR9.5**: Standardize log message format and structured logging patterns

### FR10: Configuration Consistency
59. **FR10.1**: Ensure all New* functions have consistent parameter order (config, logger, dependencies)
60. **FR10.2**: Standardize error handling patterns
61. **FR10.3**: Maintain YAML config format but potentially restructure content

## Non-Goals (Out of Scope)

1. **Performance optimization**: This refactor focuses on code quality, not performance
2. **API changes**: No changes to public APIs or command-line interfaces
3. **Feature additions**: No new features during refactoring process
4. **Architecture redesign**: No changes to overall system architecture
5. **Test additions**: Focus on cleaning up existing code, not writing new tests
6. **Database schema changes**: No changes to data storage structures
7. **External API modifications**: No changes to webhook or external service integrations

## Technical Considerations

- **Backward Compatibility**: Each phase must ensure functionality remains unchanged
- **Phase-based Approach**: Refactor by module for easier review and testing
- **Go Module Dependencies**: Use `go mod tidy` and `go mod why` to check dependencies
- **Logging Framework**: Continue using zerolog but optimize usage patterns
- **Configuration**: Maintain YAML format with potential schema restructuring
- **Context Patterns**: Use context.Context consistently for cancellation and timeouts management
- **Error Handling**: Maintain error wrapping patterns from internal/common/errors.go
- **HTTP Client Reuse**: Maximize reuse of common/http_client.go across all network operations
- **URL Processing**: Centralize all URL operations in urlhandler package for consistency

## Success Metrics

1. **Code Duplication**: Reduce duplicate code by at least 40% (measured by tools like gocyclo or manual review)
2. **Comment Density**: Reduce comment-to-code ratio to below 15%
3. **Dependencies**: Remove at least 10-15 unused dependencies
4. **Log Volume**: Reduce Info-level log messages by 50% in normal operations
5. **Build Time**: Maintain or improve build time after dependency removal
6. **Functionality**: 100% functional tests pass after each phase
7. **Context Cancellation**: 100% of long-running operations properly support context cancellation
8. **Error Consistency**: 90% of errors use standardized error types from internal/common
9. **Common Package Usage**: 80% of packages use common utilities instead of duplicate implementations
10. **Function Complexity**: Average function length reduced by 30% for target files
11. **URL Operations**: 100% of URL operations centralized in urlhandler package
12. **Discord Message Consistency**: 100% of Discord messages follow standardized color and format scheme

## Open Questions

1. **Shared Packages**: Should we create additional internal/common packages for shared utilities?
2. **Error Handling**: Should we standardize error wrapping patterns further?
3. **Interface Extraction**: Which interfaces should be extracted to improve testability?
4. **Config Restructuring**: Should config structure be optimized while maintaining YAML format?
5. **Testing Strategy**: How to ensure refactoring doesn't break functionality without comprehensive tests?
6. **Service Lifecycle**: Should we create a shared service lifecycle manager?
7. **Context Patterns**: Should we create shared context utilities for common patterns like timeout and cancellation?
8. **HTTP Client Strategy**: Should we create specialized HTTP client factories for different use cases?
9. **Model Versioning**: How to handle model changes during refactoring while maintaining backward compatibility?
10. **Discord Message Templates**: Should we create a template system for Discord message generation? 