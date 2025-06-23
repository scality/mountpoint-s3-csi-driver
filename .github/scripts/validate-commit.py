#!/usr/bin/env python3
"""
Advanced commit message validation for conventional commits with issue ID support.

This script validates commit messages according to the Conventional Commits specification
with additional requirements for issue IDs on user-facing changes.

Requirements:
- User-facing commits (feat, fix, perf, security, breaking) MUST have issue IDs
- Development commits (docs, test, ci, chore, refactor, style, build) MAY have issue IDs
- Issue ID format: S3CSI-123 or GitHub issue references
- Commit message format: type(scope): description
"""

import re
import sys
import argparse
from typing import Optional, Tuple, List


class CommitValidator:
    """Validates commit messages according to conventional commit standards."""

    # User-facing commit types that REQUIRE issue IDs
    USER_FACING_TYPES = {'feat', 'fix', 'perf', 'security', 'breaking'}

    # Development commit types where issue IDs are OPTIONAL
    DEVELOPMENT_TYPES = {'docs', 'test', 'ci', 'chore', 'refactor', 'style', 'build', 'revert'}

    # All valid commit types
    ALL_TYPES = USER_FACING_TYPES | DEVELOPMENT_TYPES

    # Issue ID patterns
    JIRA_PATTERN = r'S3CSI-\d+'
    GITHUB_ISSUE_PATTERN = r'#\d+'

    # Conventional commit pattern
    COMMIT_PATTERN = re.compile(
        r'^(?P<type>\w+)(?:\((?P<scope>[^)]+)\))?: (?P<description>.+)$'
    )

    def __init__(self):
        self.errors = []
        self.warnings = []

    def validate_commit_message(self, message: str) -> Tuple[bool, List[str], List[str]]:
        """
        Validate a commit message.

        Returns:
            Tuple of (is_valid, errors, warnings)
        """
        self.errors = []
        self.warnings = []

        # Split message into lines and filter out comments
        lines = [line for line in message.strip().split('\n') if line.strip() and not line.strip().startswith('#')]
        if not lines:
            self.errors.append("Commit message cannot be empty (ignoring template comments)")
            return False, self.errors, self.warnings

        subject_line = lines[0]

        # Validate basic format
        match = self.COMMIT_PATTERN.match(subject_line)
        if not match:
            self.errors.append(
                f"Invalid commit format. Expected: type(scope): description\n"
                f"Got: {subject_line}\n"
                f"Examples:\n"
                f"  feat(S3CSI-123): add custom S3 endpoint support\n"
                f"  fix(S3CSI-456): resolve authentication timeout issue\n"
                f"  docs: update installation guide"
            )
            return False, self.errors, self.warnings

        commit_type = match.group('type').lower()
        scope = match.group('scope') or ''
        description = match.group('description')

        # Validate commit type
        if commit_type not in self.ALL_TYPES:
            self.errors.append(
                f"Invalid commit type '{commit_type}'. "
                f"Valid types: {', '.join(sorted(self.ALL_TYPES))}"
            )
            return False, self.errors, self.warnings

        # Validate description length
        if len(description) < 10:
            self.errors.append("Description must be at least 10 characters long")

        if len(description) > 72:
            self.warnings.append("Description should be 72 characters or less for better readability")

        # Validate issue ID requirements
        self._validate_issue_id(commit_type, scope, description, message)

        # Additional validations
        self._validate_description_style(description)

        return len(self.errors) == 0, self.errors, self.warnings

    def _validate_issue_id(self, commit_type: str, scope: str, description: str, full_message: str):
        """Validate issue ID requirements based on commit type."""
        has_jira_id = bool(re.search(self.JIRA_PATTERN, scope)) if scope else False
        has_github_issue = bool(re.search(self.GITHUB_ISSUE_PATTERN, full_message))
        has_issue_id = has_jira_id or has_github_issue

        if commit_type in self.USER_FACING_TYPES:
            if not has_issue_id:
                self.errors.append(
                    f"User-facing commit type '{commit_type}' requires an issue ID.\n"
                    f"Include issue ID in scope: {commit_type}(S3CSI-123): description\n"
                    f"Or reference GitHub issue in footer: Closes #123"
                )
            elif has_jira_id and not re.match(r'^S3CSI-\d+$', scope):
                self.errors.append(
                    f"Invalid JIRA issue format in scope. Expected: S3CSI-123, got: {scope}"
                )
        elif commit_type in self.DEVELOPMENT_TYPES:
            # Issue ID is optional for development commits
            if scope and not (has_jira_id or scope.replace('-', '').replace('_', '').isalnum()):
                self.warnings.append(
                    f"Scope '{scope}' doesn't look like an issue ID or component name"
                )

    def _validate_description_style(self, description: str):
        """Validate description style guidelines."""
        # Should start with lowercase (imperative mood)
        if description and description[0].isupper():
            self.warnings.append(
                "Description should start with lowercase letter (imperative mood). "
                "E.g., 'add feature' not 'Add feature'"
            )

        # Should not end with period
        if description.endswith('.'):
            self.warnings.append("Description should not end with a period")

        # Should be imperative mood
        if any(description.lower().startswith(word) for word in ['added', 'fixed', 'updated', 'changed']):
            self.warnings.append(
                "Use imperative mood: 'add' not 'added', 'fix' not 'fixed'"
            )


def main():
    """Main entry point for commit message validation."""
    parser = argparse.ArgumentParser(description='Validate conventional commit messages')
    parser.add_argument('commit_file', nargs='?', help='File containing commit message')
    parser.add_argument('--message', '-m', help='Commit message to validate')
    parser.add_argument('--strict', action='store_true', help='Treat warnings as errors')

    args = parser.parse_args()

    # Get commit message
    if args.message:
        commit_message = args.message
    elif args.commit_file:
        try:
            with open(args.commit_file, 'r', encoding='utf-8') as f:
                commit_message = f.read()
        except FileNotFoundError:
            print(f"❌ Error: Commit file '{args.commit_file}' not found")
            sys.exit(1)
        except Exception as e:
            print(f"❌ Error reading commit file: {e}")
            sys.exit(1)
    else:
        # Read from stdin
        commit_message = sys.stdin.read()

    # Validate commit message
    validator = CommitValidator()
    is_valid, errors, warnings = validator.validate_commit_message(commit_message)

    # Print results
    if errors:
        print("❌ Commit message validation failed:")
        for error in errors:
            print(f"   {error}")
        print()

    if warnings:
        print("⚠️  Commit message warnings:")
        for warning in warnings:
            print(f"   {warning}")
        print()

    if is_valid and not warnings:
        print("✅ Commit message is valid!")
    elif is_valid:
        print("✅ Commit message is valid (with warnings)")

    # Exit with appropriate code
    if not is_valid or (args.strict and warnings):
        print("\n📚 Conventional Commit Examples:")
        print("   feat(S3CSI-123): add custom S3 endpoint support")
        print("   fix(S3CSI-456): resolve authentication timeout issue")
        print("   docs: update installation guide")
        print("   chore(deps): bump mkdocs from 1.5.0 to 1.6.0")
        print("   breaking(S3CSI-789): remove deprecated API endpoints")
        sys.exit(1)

    sys.exit(0)


if __name__ == '__main__':
    main()
