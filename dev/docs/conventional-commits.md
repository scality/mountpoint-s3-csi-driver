# Conventional Commits Guide

This guide explains how to write conventional commits for the Scality S3 CSI Driver project.

## What are Conventional Commits

[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) provide a structured format for commit messages that enables automated changelog generation, semantic versioning,
and better project history tracking.

Our project uses conventional commits to automatically generate changelogs and determine version bumps based on the types of changes made.

## Commit Message Format

```text
<type>(<scope>): <description>

[optional body]

[optional footer]
```

All parts explained below:

### Type (Required)

**User-facing changes (require issue ID):**

- `feat` - New feature for users
- `fix` - Bug fix that affects users
- `perf` - Performance improvement
- `security` - Security-related changes
- `breaking` - Breaking changes (also add `BREAKING CHANGE:` in footer)

**Development changes (issue ID optional):**

- `docs` - Documentation changes
- `test` - Adding or updating tests
- `ci` - CI/CD pipeline changes
- `chore` - Maintenance tasks, dependency updates

### Scope (Conditional)

**Issue ID Requirements:**

- Format: `S3CSI-123` (Jira ticket number)
- Required for: feat, fix, perf, security, breaking
- Optional for: docs, test, ci, chore

**Examples:**

- Issue ID: `S3CSI-123`
- Component: `controller`, `node`, `helm`, `docs`

### Description (Required)

- Use imperative mood: "add" not "added" or "adds"
- Start with lowercase letter
- No period at the end
- Maximum 50 characters
- Be concise but descriptive

## Examples

### User-Facing Changes (Require Issue ID)

```bash
feat(S3CSI-123): add custom S3 endpoint support
fix(S3CSI-456): resolve authentication timeout with RING
perf(S3CSI-789): optimize mount performance for large files
security(S3CSI-321): update dependencies to fix CVE-2023-1234
```

### Development Changes (Issue ID Optional)

```bash
docs: update installation guide
test: add unit tests for volume controller
ci: add automated security scanning
chore(deps): bump mkdocs from 1.5.0 to 1.6.0
```

### Breaking Changes

```bash
breaking(S3CSI-555): remove deprecated mount options

BREAKING CHANGE: The legacy mount options `cache-dir` and `cache-size`
have been removed. Use `local-cache-dir` and `local-cache-size` instead.
```

### With Body and Footer

```bash
feat(S3CSI-123): add support for custom S3 endpoints

This change allows users to specify custom S3-compatible endpoints
for use with private cloud storage solutions like RING.

The implementation includes:
- New volumeContext parameter: customEndpoint
- Validation of endpoint URLs
- Integration with existing authentication mechanisms

Closes S3CSI-123
Refs S3CSI-100
```

## Issue ID Requirements

### Required for User-Facing Changes

These commit types **must** include an issue ID because they affect users and appear in release notes:

- `feat` - New features
- `fix` - Bug fixes
- `perf` - Performance improvements
- `security` - Security updates
- `breaking` - Breaking changes

### Optional for Development Changes

These commit types **may** include an issue ID but it's not required:

- `docs` - Documentation updates
- `test` - Test additions/updates
- `ci` - CI/CD changes
- `chore` - Maintenance tasks

## How to Create Commits

### Option 1: Using Commitizen (Recommended)

After making your changes, use the interactive commit tool:

```bash
# Interactive commit creation
cz commit
```

Commitizen will guide you through:

1. Selecting commit type
2. Entering scope/issue ID
3. Writing description
4. Adding body and footer if needed

### Option 2: Manual Commits

If you prefer writing commits manually:

```bash
git commit -m "feat(S3CSI-123): add custom S3 endpoint support"
```

### Option 3: Git Commit Template

Use the provided commit template:

```bash
# Set up the template (done automatically by setup script)
git config commit.template .github/commit-template.txt

# Create commits with template
git commit
```

## Validation and Pre-commit Hooks

Our pre-commit hooks automatically validate commit messages. If validation fails:

1. Read the error message carefully
2. Fix the commit message format
3. Try committing again

Common validation errors:

### Missing Issue ID

```bash
# Error: feat commits require issue ID in scope
# Wrong: feat: add custom S3 endpoint support
# Fix:
git commit --amend -m "feat(S3CSI-123): add custom S3 endpoint support"
```

### Invalid Format

```bash
# Error: commit doesn't follow type(scope): description format
# Wrong: Add custom S3 endpoint support
# Fix:
git commit --amend -m "feat(S3CSI-123): add custom S3 endpoint support"
```

### Description Too Short

```bash
# Error: description must be at least 10 characters long
# Wrong: feat(S3CSI-123): add support
# Fix:
git commit --amend -m "feat(S3CSI-123): add comprehensive S3 endpoint support"
```

### Subject Line Too Long

```bash
# Error: subject line exceeds 55 characters for optimal Git log readability
# Wrong: feat(S3CSI-123): add comprehensive S3 endpoint support with advanced configuration options
# Fix:
git commit --amend -m "feat(S3CSI-123): add comprehensive S3 endpoint support"
```

## IDE Integration

### VS Code

The setup script configures VS Code with:

- Conventional Commits extension
- Git commit message templates
- Automated formatting on save

### Other IDEs

For other IDEs, manually configure:

- Git commit template: `.github/commit-template.txt`
- Pre-commit hooks will still validate commits

## Migration Strategy

For existing contributors:

- **New commits** MUST follow conventional format
- **Existing commits** don't need to be changed
- **Squash commits** in PRs to ensure clean history
- **Release notes** will be generated from conventional commits going forward

This ensures a smooth transition while maintaining project history.
