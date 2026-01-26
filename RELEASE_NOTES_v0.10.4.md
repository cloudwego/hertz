# Release Notes for v0.10.4

## What's Changed

### Chore

#### [[#1477](https://github.com/cloudwego/hertz/pull/1477)] chore: update deps for supporting go1.26
**Summary**: Updates project dependencies to support Go 1.26 and newer versions of critical libraries.

**Changes**:
- Updates `github.com/bytedance/gopkg` from v0.1.1 to v0.1.3 - includes performance improvements and bug fixes
- Updates `github.com/bytedance/sonic` from v1.14.0 to v1.15.0 - the high-performance JSON serialization library with Go 1.26 compatibility
- Updates `github.com/bytedance/sonic/loader` from v0.3.0 to v0.5.0 - improved JIT compilation support
- Updates `github.com/cloudwego/netpoll` from v0.7.0 to v0.7.2 - network polling library improvements
- Updates `github.com/cloudwego/base64x` from v0.1.5 to v0.1.6 - optimized base64 encoding/decoding
- Updates `github.com/klauspost/cpuid/v2` from v2.0.9 to v2.2.9 - better CPU feature detection
- Updates `github.com/stretchr/testify` from v1.9.0 to v1.10.0 - testing framework improvements

**Impact**: Ensures Hertz is compatible with Go 1.26 and benefits from performance improvements and bug fixes in dependencies.

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

This release focuses on:

1. **Go 1.26 Support**: Updated all dependencies to support the latest Go version
2. **Release Automation**: Added comprehensive scripts for managing releases and hotfixes
3. **Repository Modernization**: Consolidated repository structure, brought cmd/hz into main repo
4. **Enhanced CI/CD**: Improved workflows for testing, security scanning, and code quality
5. **Better Developer Experience**: Added comprehensive tooling, documentation, and community guidelines

This is primarily a maintenance and infrastructure release that sets the foundation for better development practices and smoother releases going forward.
