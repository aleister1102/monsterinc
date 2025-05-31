# PRD: Full Codebase Refactoring (Task 15)

## 1. Introduction/Overview

This document outlines the requirements for refactoring the existing Go codebase of the MonsterInc project. The primary goal is to improve code readability, enhance maintainability, and prepare the codebase for future feature development by applying clean code principles as defined in the project's refactoring guidelines.

## 2. Goals

*   **Improve Readability:** Make the code easier to understand for all developers, reducing cognitive load and onboarding time.
*   **Enhance Maintainability:** Structure the code such that future modifications, bug fixes, and feature additions can be implemented more efficiently and with lower risk.
*   **Prepare for Future Features:** Ensure the codebase is clean and modular enough to support upcoming functionalities without requiring extensive rework.
*   **Reduce Code Complexity:**
    *   Minimize the number of lines of code (LoC) per function.
    *   Reduce the number of parameters per function to the optimal minimum (ideally 0-2, or grouped into structs).
*   **Optimize Logging:** Reduce "noisy" informational logs (`logger.Info()`), ensuring they provide only essential, high-value information, while maintaining detail in debug and error logs.

## 3. User Stories

*(From a developer's perspective)*

*   As a developer, I want to quickly understand the purpose, inputs, outputs, and core logic of any function so that I can debug issues or add functionality efficiently.
*   As a developer, I want the codebase to have a consistent style, naming conventions, and structure so that context switching between different parts of the code is seamless.
*   As a developer, I want application logs at the `INFO` level to be concise and meaningful, so I can quickly identify key operational events without sifting through excessive non-critical details.
*   As a developer, I want to be confident that changes in one part of the code have minimal unintended side effects on other parts.

## 4. Functional Requirements

1.  **Apply Clean Code Principles:** All Go files within the `internal/` and `cmd/` directories of the project must be refactored according to the guidelines specified in `.cursor/rules/refactor.mdc`.
2.  **Prioritized Files for Refactoring:** Special attention and priority should be given to refactoring the following core files and their closely related helper/utility files within the same package:
    *   `internal/orchestrator/orchestrator.go`
    *   `internal/scheduler/scheduler.go` (and by implication, `db.go`, `target_manager.go` in the same package)
    *   `internal/crawler/crawler.go` (and by implication, `asset.go`, `scope.go` in the same package)
    *   `internal/httpxrunner/runner.go` (and by implication, `result.go` in the same package)
    *   `internal/reporter/html_reporter.go`
    *   `internal/reporter/html_diff_reporter.go`
    *   `internal/datastore/parquet_reader.go`
    *   `internal/datastore/parquet_writer.go`
    *   `internal/monitor/service.go` (and by implication, `fetcher.go`, `processor.go` in the same package)
3.  **Function Length:** Functions should be concise and do one thing well. Reduce the number of lines of code per function where appropriate, without sacrificing clarity.
4.  **Function Parameters:** Minimize the number of parameters for each function. Aim for 0, 1, or 2 parameters. If more are necessary, group them into a dedicated struct.
5.  **Logging Refinement:**
    *   Review all `logger.Info()` calls throughout the codebase.
    *   Modify or remove informational logs that are overly verbose, redundant, or do not provide significant value for understanding the application's operational state.
    *   Ensure `logger.Debug()` is used for detailed diagnostic information and `logger.Error()`, `logger.Fatal()` for actual error conditions.
6.  **Functional Equivalence:** The refactored code must maintain the existing functionality of the application. No features should be unintentionally altered or broken.
7.  **Code Formatting:** Ensure all Go code is formatted using `gofmt` or `goimports`.

## 5. Non-Goals (Out of Scope)

*   **Major Architectural Changes:** This refactoring effort will not involve large-scale architectural redesigns (e.g., transitioning to microservices, overhauling module boundaries extensively) unless a minor adjustment is a natural and necessary outcome of applying clean code principles to a prioritized file.
*   **New Feature Implementation:** No new user-facing features will be added as part of this task.
*   **Dedicated Performance Optimization:** Performance improvements are not a primary goal, though they may occur as a side effect of code simplification.
*   **Dependency Updates:** Updating external libraries or Go modules to their latest versions is not in scope, unless an existing version directly hinders the refactoring process or causes issues related to code clarity.
*   **Test Creation/Expansion:** Writing new unit tests or increasing test coverage is outside the scope of this PRD. (User will handle testing).

## 6. Design Considerations (Optional)

*   Strict adherence to the principles and guidelines outlined in `.cursor/rules/refactor.mdc` is paramount.
*   When reducing parameters, consider if introducing a small, well-named struct improves clarity more than passing many individual parameters.

## 7. Technical Considerations (Optional)

*   Refactoring should ideally be performed incrementally (e.g., package by package, or file by file for the prioritized list) to manage complexity and allow for easier review.
*   Pay close attention to public interfaces and exported functions/types to ensure that contracts with other parts of the system are maintained.
*   Changes to logging should be carefully considered to ensure that critical information is not lost.

## 8. Success Metrics

*   **Code Review Acceptance:** All refactoring changes are approved through code review, with reviewers confirming improved readability, maintainability, and adherence to `refactor.mdc`.
*   **Reduced Complexity Metrics (Qualitative & Quantitative):**
    *   Observable reduction in the average number of lines per function in refactored modules.
    *   Observable reduction in the average number of parameters per function in refactored modules.
*   **Developer Feedback:** Positive qualitative feedback from developers indicating that the refactored code is easier to understand, work with, and debug.
*   **Log Clarity:** Application logs at the `INFO` level are noticeably less verbose and more focused on essential operational events.
*   **Functional Stability:** No new bugs or regressions are introduced as a result of the refactoring, as verified by the user through their testing processes.

## 9. Open Questions

*   Are there specific quantitative targets for "noisy info logs" (e.g., desired reduction in log lines per common operation, or specific log messages to eliminate)? (Currently, this will be a qualitative judgment based on "only show things that are really necessary"). 