# Release Notes for v0.10.4

## What's Changed

### Chore

#### [[#1477](https://github.com/cloudwego/hertz/pull/1477)] chore: update deps for supporting go1.26
**Summary**: Updates project dependencies to support Go 1.26 and newer versions of critical libraries.

**Changes**:
- Updates `github.com/bytedance/gopkg` from v0.1.1 to v0.1.3
  - Performance improvements and bug fixes
  
- Updates `github.com/bytedance/sonic` from v1.14.0 to v1.15.0
  - **Go 1.26 compatibility support** 
  - Avoids boundary pointer issues in string quoting
  - Adds fallback implementations for unquoting and UTF-8 validation
  - Fixes panic when encoding unsupported map-key types
  - Fixes range check for uint32 in JIT
  - Fixes bugs with encoding.TextMarshaler keys
  - Fixes decode of JSON containing \u0000 characters
  - Optimizations for encode and AST node performance
  - Shows JSON trace when panic occurs for better debugging
  
- Updates `github.com/bytedance/sonic/loader` from v0.3.0 to v0.5.0
  - Go 1.26 support with improved JIT compilation
  - PCSP (Program Counter Stack Pointer) data for JIT functions
  
- Updates `github.com/cloudwego/netpoll` from v0.7.0 to v0.7.2
  - Fixes MallocAck logic for discarding rest data
  - Removes unused zero-copy code
  - Lint fixes and code cleanup
  
- Updates `github.com/cloudwego/base64x` from v0.1.5 to v0.1.6
  - Optimized base64 encoding/decoding performance
  
- Updates `github.com/klauspost/cpuid/v2` from v2.0.9 to v2.2.9
  - Better CPU feature detection across architectures
  
- Updates `github.com/stretchr/testify` from v1.9.0 to v1.10.0
  - Latest testing framework improvements and bug fixes

**Impact**: 
- **Critical**: Enables Hertz to work with Go 1.26, ensuring users can upgrade to the latest Go version
- Improves JSON serialization performance and reliability through sonic updates
- Enhances network I/O stability through netpoll fixes
- Better error diagnostics with improved panic traces

#### [[#1476](https://github.com/cloudwego/hertz/pull/1476)] chore: add scripts for release
**Summary**: Adds automated release management scripts to streamline the release process.

**Changes**:
- Adds `scripts/release.sh` - main release script for creating version tags
  - Validates branch and commit status
  - Shows changes since last version
  - Supports major/minor/patch version bumps
  - Validates version.go consistency
  - Creates and pushes release tags
  - Includes dry-run mode for testing
  
- Adds `scripts/release-hotfix.sh` - hotfix release script for patch versions
  - Supports creating hotfix branches from specific minor versions
  - Manages hotfix branch lifecycle
  - Creates patch version tags from hotfix branches
  - Includes dry-run mode

- Adds `scripts/.utils/funcs.sh` - shared utility functions
  - Version increment utilities
  - Change diff visualization
  - Commit validation helpers
  
- Adds `scripts/.utils/check_version.sh` - version consistency checker
  - Validates version.go matches release version
  - Interactive confirmation for mismatches
  
- Adds `scripts/.utils/check_go_mod.sh` - dependency validator
  - Ensures CloudWeGo dependencies use semantic versioning
  - Prevents releases with non-standard dependency versions

- Updates `version.go` from "v0.10.3" to "v0.10.3-dev" to indicate development state

- Removes legacy pre-commit hooks:
  - Removes `script/go-fmt`
  - Removes `script/pre-commit-hook`

**Impact**: Significantly improves release process automation and consistency. Reduces human error in releases and ensures proper versioning practices.

#### [[#1475](https://github.com/cloudwego/hertz/pull/1475)] Deprecating develop branch
**Summary**: Merges develop branch into main, consolidating the codebase and bringing cmd/hz tool into the main repository structure.

**Changes**:
- **Repository Structure**:
  - Adds comprehensive GitHub configuration files (.codecov.yml, .gitattributes)
  - Adds CODEOWNERS file for automatic PR review assignments
  - Adds issue templates (bug_report.md, feature_request.md, question.md)
  - Updates PR template with better guidelines
  - Adds labels.json for GitHub issue/PR labels

- **GitHub Workflows**:
  - Adds `.github/workflows/unit-tests.yml` - Enhanced unit test workflow with coverage reporting
  - Adds `.github/workflows/cmd-tests.yml` - Dedicated cmd/hz tool testing workflow
  - Adds `.github/workflows/pr-check.yml` - PR validation checks
  - Adds `.github/workflows/vulncheck.yml` - Security vulnerability scanning
  - Adds `.github/workflows/labeler.yml` - Automatic PR labeling
  - Adds `.github/workflows/invalid_question.yml` - Invalid issue management
  - Updates unit test coverage collection to use `-coverpkg=./...` for more accurate coverage

- **cmd/hz Tool Integration**:
  - Brings the complete cmd/hz code generation tool into main repository under `cmd/hz/`
  - Includes full protobuf and thrift IDL support
  - Generator for handlers, models, routers, and clients
  - Test infrastructure for cmd/hz (test_hz_unix.sh, test_hz_windows.sh)
  - Test data and fixtures for validation

- **Build and Development**:
  - Adds comprehensive Makefile with common development tasks
  - Adds .golangci.yaml linting configuration
  - Adds .licenserc.yaml for license header checking
  - Adds .typos.toml for typo checking
  - Updates .gitignore with better coverage

- **Documentation**:
  - Adds CODE_OF_CONDUCT.md
  - Adds CONTRIBUTING.md with contribution guidelines
  - Enhances README.md and README_cn.md
  - Adds ROADMAP.md
  - Adds LICENSE and NOTICE files

**Impact**: 
- Simplifies repository management by consolidating from dual-branch (main/develop) to single-branch (main) workflow
- Makes cmd/hz tool an integral part of the main Hertz repository, improving maintainability
- Enhances developer experience with better tooling, automation, and documentation
- Improves code quality through enhanced CI/CD workflows and automated checks
- Provides better community engagement through standardized templates and guidelines

#### [[#1478](https://github.com/cloudwego/hertz/pull/1478)] chore: prepare for v0.10.4 release
**Summary**: Final preparation commit consolidating all changes for v0.10.4 release.

**Full Changelog**: https://github.com/cloudwego/hertz/compare/v0.10.3...v0.10.4

---

## Summary

This release focuses on infrastructure improvements and Go 1.26 support:

### Key Highlights

1. **Go 1.26 Compatibility** ✅
   - Updated all critical dependencies (sonic, netpoll, gopkg) to support Go 1.26
   - Users can now safely upgrade to the latest Go version

2. **Release Process Automation** 🚀
   - Added comprehensive scripts for managing releases and hotfixes
   - Dry-run mode for safe testing
   - Automated version validation and dependency checking
   - Reduces human error and ensures consistency

3. **Repository Consolidation** 🔄
   - Merged develop branch into main for simplified workflow
   - Brought cmd/hz code generation tool into main repository
   - Single-branch workflow improves maintainability

4. **Enhanced CI/CD** 🔧
   - New workflows for cmd testing, security scanning, and PR validation
   - Improved code coverage collection
   - Automated labeling and issue management

5. **Better Developer Experience** 📚
   - Comprehensive GitHub templates and issue forms
   - Clear contribution guidelines
   - Improved documentation and code standards

### Performance & Reliability Improvements

From dependency updates:
- **JSON Performance**: sonic v1.15.0 brings encoding optimizations and better error handling
- **Network Stability**: netpoll v0.7.2 fixes memory management issues
- **Better Debugging**: Improved panic traces show JSON context

### Breaking Changes

None - this is a fully backward-compatible release.

### Upgrade Recommendation

**Recommended for all users**, especially those wanting to:
- Use Go 1.26
- Benefit from JSON performance improvements
- Take advantage of enhanced tooling and automation

This release sets the foundation for improved development velocity and code quality in future releases.
