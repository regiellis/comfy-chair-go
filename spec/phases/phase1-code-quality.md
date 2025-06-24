# Phase 1: Code Quality & Foundation

## Overview
Establish a solid foundation for future development by cleaning up technical debt, adding testing infrastructure, and improving code maintainability.

## Objectives
- Remove unused code and resolve linter warnings
- Continue refactoring main.go for better organization
- Establish comprehensive testing framework
- Add basic CLI enhancements

## Tasks

### 1. Code Cleanup

#### Remove Unused Functions
**Priority**: High  
**Effort**: Low  

Functions identified by linter as unused:
- `saveComfyUIPathToEnv` (main.go:285)
- `checkVenvPython` (main.go:309) 
- `cleanupStaleEntries` (main.go:1014)
- `getCachedStatus` (main.go:1033)
- `updateCache` (main.go:1041)
- `isProcessRunningReal` (main.go:1052)

**Implementation**:
1. Verify functions are truly unused with code analysis
2. Remove functions and any related imports
3. Test build to ensure no breakage
4. Commit with clear message about cleanup

#### Fix Linter Suggestions
**Priority**: Medium  
**Effort**: Low

- Fix tagged switch suggestion (main.go:444)
- Address any other modernization suggestions

### 2. Continued Refactoring

#### Extract More Modules from main.go
**Priority**: High  
**Effort**: Medium

**Current**: 2368 lines  
**Target**: < 2000 lines (extract ~400 lines)

**Modules to Extract**:

1. **`internal/menu.go`** (~200 lines)
   - Interactive menu system
   - Menu navigation logic
   - CLI argument parsing

2. **`internal/migration.go`** (~150 lines)
   - `migrateInputImages`
   - `migrateWorkflows` 
   - `migrateCustomNodes`
   - Migration utilities

3. **`internal/operations.go`** (~100 lines)
   - Environment management operations
   - Status reporting functions
   - Cleanup operations

**Implementation Steps**:
1. Create new module files
2. Move functions with minimal dependencies first
3. Update imports and function calls
4. Test build and functionality after each module
5. Update CLAUDE.md with new architecture

### 3. Testing Infrastructure

#### Set Up Testing Framework
**Priority**: High  
**Effort**: Medium

**Test Structure**:
```
/tests/
  unit/
    internal/
      core_test.go
      process_test.go
      utils_test.go
    main_test.go
    nodes_test.go
  integration/
    workflow_test.go
    environment_test.go
  fixtures/
    test_configs/
    mock_comfyui/
```

#### Unit Tests (Target: 60% coverage)
**Priority**: High  
**Effort**: High

**Core Functions to Test**:

1. **Environment Management** (`internal/core_test.go`)
   - `GetActiveComfyInstall()` with various config states
   - `RunWithEnvConfirmation()` error handling
   - Configuration loading/validation

2. **Process Management** (`internal/process_test.go`)
   - PID file operations
   - Process status detection
   - Command execution utilities

3. **Node Operations** (`nodes_test.go`)
   - Node creation and templating
   - Template placeholder replacement
   - Input validation and sanitization

4. **Utilities** (`internal/utils_test.go`)
   - Path expansion and normalization
   - Configuration file handling
   - Error handling utilities

#### Integration Tests
**Priority**: Medium  
**Effort**: Medium

**Test Scenarios**:
- Complete node creation workflow
- Environment setup and validation
- Configuration migration
- Error recovery scenarios

#### CI/CD Setup
**Priority**: Medium  
**Effort**: Low

- GitHub Actions workflow for automated testing
- Test coverage reporting
- Build verification on multiple platforms

### 4. Basic CLI Enhancements

#### Version Information
**Priority**: Medium  
**Effort**: Low

Add `--version` flag with build information:
```bash
comfy-chair --version
# Output:
# Comfy Chair v1.3.3
# Build: abc1234 (2024-01-15T10:30:00Z)
# Go: go1.21.5
# Platform: linux/amd64
```

**Implementation**:
- Add version variables to main.go
- Use ldflags to inject build info
- Update help text and CLI parsing

#### Improved Error Messages
**Priority**: Medium  
**Effort**: Medium

**Current Issues**:
- Generic error messages
- No actionable guidance
- Inconsistent formatting

**Improvements**:
- Structured error types with context
- Actionable suggestions in error messages
- Consistent error formatting
- Links to documentation where helpful

#### Structured Logging
**Priority**: Low  
**Effort**: Medium

Replace `fmt.Print*` with structured logging:
- Configurable log levels (DEBUG, INFO, WARN, ERROR)
- JSON output option for automation
- File output option
- Colored terminal output

#### Shell Completion
**Priority**: Low  
**Effort**: Medium

Add autocompletion for:
- bash
- zsh  
- fish

Generate completion scripts and installation instructions.

## Implementation Order

1. **Code Cleanup** (1-2 days)
   - Remove unused functions
   - Fix linter warnings
   - Clean commit

2. **Testing Setup** (3-4 days)
   - Create test structure
   - Add core unit tests
   - Set up CI pipeline

3. **Refactoring** (3-4 days)
   - Extract menu module
   - Extract migration module
   - Update documentation

4. **CLI Enhancements** (2-3 days)
   - Add version flag
   - Improve error messages
   - Add shell completion

## Acceptance Criteria

### Code Quality
- [ ] All linter warnings resolved
- [ ] No unused functions or dead code
- [ ] main.go < 2000 lines
- [ ] Clean, well-organized module structure

### Testing
- [ ] Unit test coverage > 60%
- [ ] All tests pass consistently
- [ ] CI pipeline validates builds
- [ ] Integration tests cover critical workflows

### CLI Experience
- [ ] Version information available
- [ ] Consistent error message formatting
- [ ] Shell completion works correctly
- [ ] Help text is comprehensive and clear

### Documentation
- [ ] CLAUDE.md updated with new architecture
- [ ] Code is well-commented
- [ ] Test documentation explains coverage
- [ ] Migration guide for any breaking changes

## Risk Mitigation

### Refactoring Risks
- **Risk**: Breaking existing functionality during module extraction
- **Mitigation**: 
  - Extract small modules incrementally
  - Test thoroughly after each extraction
  - Maintain backward compatibility

### Testing Complexity
- **Risk**: Tests become overly complex or brittle
- **Mitigation**:
  - Start with simple, focused unit tests
  - Use test fixtures and mocks appropriately
  - Focus on testing behavior, not implementation

### Time Estimation
- **Risk**: Tasks take longer than estimated
- **Mitigation**:
  - Prioritize highest-impact items first
  - Break large tasks into smaller chunks
  - Regular progress checkpoints

## Success Metrics

- Main.go reduced by 15%+ (400+ lines extracted)
- Test coverage established at 60%+
- Zero linter warnings
- Build time maintained or improved
- All existing functionality preserved
- Foundation ready for Phase 2 development