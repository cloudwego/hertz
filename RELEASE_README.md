# v0.10.4 Release Documentation

This directory contains the release notes for Hertz v0.10.4.

## Files

### GITHUB_RELEASE_NOTES.md
This file contains the concise release notes in the format used for GitHub releases. It follows the same style as previous releases (e.g., v0.10.3) and can be directly copied into the GitHub release page.

**Usage**: Copy the contents of this file directly into the GitHub release description when creating the v0.10.4 release.

### RELEASE_NOTES_v0.10.4.md  
This file contains the comprehensive, detailed release notes with:
- Detailed analysis of each PR
- Specific changes in dependencies  
- Code-level impact analysis
- Performance and reliability improvements
- Migration guide
- Summary of key highlights

**Usage**: Use this for:
- Internal documentation
- Detailed changelog reference
- Understanding the technical impact of each change
- Blog posts or announcements

## Release Summary

**v0.10.4** is an infrastructure and compatibility release that includes:

1. **Go 1.26 Support** - All dependencies updated for Go 1.26 compatibility
2. **Release Automation** - New scripts for managing releases and hotfixes
3. **Repository Consolidation** - Merged develop branch, brought cmd/hz into main repo
4. **Enhanced CI/CD** - New workflows for testing, security, and automation
5. **Developer Experience** - Better templates, documentation, and guidelines

This is a **backward-compatible** release recommended for all users.

## PRs Included

After v0.10.3 (released Oct 21, 2025), the following PRs were merged:

- [#1475](https://github.com/cloudwego/hertz/pull/1475) - Deprecating develop branch (Jan 16, 2026)
- [#1476](https://github.com/cloudwego/hertz/pull/1476) - Add scripts for release (Jan 16, 2026)
- [#1477](https://github.com/cloudwego/hertz/pull/1477) - Update deps for supporting go1.26 (Jan 23, 2026)
- [#1478](https://github.com/cloudwego/hertz/pull/1478) - Prepare for v0.10.4 release (Jan 23, 2026)

## Key Dependencies Updated

| Dependency | From | To | Key Changes |
|-----------|------|-----|-------------|
| sonic | v1.14.0 | v1.15.0 | Go 1.26 support, JSON encoding optimizations |
| netpoll | v0.7.0 | v0.7.2 | Memory management fixes |
| gopkg | v0.1.1 | v0.1.3 | Performance improvements |
| testify | v1.9.0 | v1.10.0 | Testing framework updates |

For full details, see the complete release notes.
