---
name: review-pr
description: Review a PR on scality/mountpoint-s3-csi-driver (Kubernetes CSI driver for S3 using Mountpoint FUSE)
argument-hint: <pr-number-or-url>
disable-model-invocation: true
allowed-tools: Read, Bash(gh repo view *), Bash(gh pr view *), Bash(gh pr diff *), Bash(gh pr comment *), Bash(gh api *), Bash(git diff *), Bash(git log *), Bash(git show *)
---

# Review GitHub PR

You are an expert code reviewer. Review this PR: $ARGUMENTS

## Determine PR target

Parse `$ARGUMENTS` to extract the repo and PR number:

- If arguments contain `REPO:` and `PR_NUMBER:` (CI mode), use those values directly.
- If the argument is a GitHub URL (starts with `https://github.com/`), extract `owner/repo` and the PR number from it.
- If the argument is just a number, use the current repo from `gh repo view --json nameWithOwner -q .nameWithOwner`.

## Output mode

- **CI mode** (arguments contain `REPO:` and `PR_NUMBER:`): post inline comments and summary to GitHub.
- **Local mode** (all other cases): output the review as text directly. Do NOT post anything to GitHub.

## Steps

1. **Fetch PR details:**

   ```bash
   gh pr view <number> --repo <owner/repo> --json title,body,headRefOid,author,files
   gh pr diff <number> --repo <owner/repo>
   ```

2. **Read changed files** to understand the full context around each change (not just the diff hunks).

3. **Analyze the changes** against these criteria:

   | Area | What to check |
   |------|---------------|
   | Error wrapping | Use `fmt.Errorf("...: %w", err)`, not `%v`. Errors must be wrappable for `errors.Is`/`errors.As`. |
   | Context propagation | Pass `context.Context` through call chains. Respect cancellation. Never store contexts in structs. |
   | Goroutine leaks | Ensure goroutines have exit conditions. Use `errgroup` for fan-out. Check channel close semantics. |
   | Kubernetes operators | Reconciler idempotency. RBAC scoping (least privilege). Status subresource updates. Proper use of expectations system for eventual consistency. |
   | Interface compliance | Check that implementations satisfy interfaces at compile time (`var _ Interface = (*Impl)(nil)`). |
   | CSI spec conformance | Volume capabilities, idempotent Create/DeleteVolume, proper gRPC error codes. |
   | CRD changes | If `pkg/api/v2/` types changed, verify `make generate` was run (deepcopy + CRD YAML). Check field selector updates in indexer. |
   | Mountpoint Pod lifecycle | Pod creation specs, security contexts, Unix socket communication, source/bind mount correctness. |
   | Credential handling | No credentials in logs or volume context. Proper secret resolution. Cleanup on unmount. |
   | Helm chart | Template syntax, default values, upgrade path. RBAC and service account changes. Run `helm lint`. |
   | Test coverage | New logic should have unit tests. Mock interfaces (Mounter, Client, PodWatcher) for isolation. Controller changes need envtest coverage. |
   | Breaking changes | Changes to CSI driver name, CRD schema, Helm values, or StorageClass parameters. |
   | Security | OWASP-relevant issues: command injection in mount args, path traversal in target paths, privilege escalation in pod specs. |

4. **Deliver your review:**

### If CI mode: post to GitHub

#### Part A: Inline file comments

For each specific issue, post a comment on the exact file and line:

```bash
gh api -X POST -H "Accept: application/vnd.github+json" "repos/<owner/repo>/pulls/<number>/comments" -f body="Your comment<br><br>— Claude Code" -f path="path/to/file" -F line=<line_number> -f side="RIGHT" -f commit_id="<headRefOid>"
```

**The command must stay on a single bash line.** Never use newlines in bash commands — use `<br>` for line breaks in comment bodies. Never put `<br>` inside code blocks or suggestion blocks.

Each inline comment must:

- Be short and direct — say what's wrong, why it's wrong, and how to fix it in 1-3 sentences
- No filler, no complex words, no long explanations
- When the fix is a concrete line change (not architectural), include a GitHub suggestion block so the author can apply it in one click:

  ````text
  ```suggestion
  corrected-line-here
  ```
  ````

  Only suggest when you can show the exact replacement. For architectural or design issues, just describe the problem.
  Example with a suggestion block:

  ```bash
  gh api ... -f body=$'Missing error wrapping.<br><br>\n```suggestion\nreturn fmt.Errorf("failed to create bucket: %w", err)\n```\n<br><br>— Claude Code' ...
  ```

- When the comment contains a suggestion block, use `$'...'` quoting with `\n` for code fence boundaries. Escape single quotes as `\'` (e.g., `don\'t`)
- End with: `— Claude Code`

Use the line number from the **new version** of the file (the line number you'd see after the PR is merged), which corresponds to the `line` parameter in the GitHub API.

#### Part B: Summary comment

```bash
gh pr comment <number> --repo <owner/repo> --body "LGTM<br><br>Review by Claude Code"
```

**The command must stay on a single bash line.** Never use newlines in bash commands — use `<br>` for line breaks in comment bodies.

Do not describe or summarize the PR. For each issue, state the problem on one line, then list one or more suggestions below it:

```text
- <issue>
  - <suggestion>
  - <suggestion>
```

If no issues: just say "LGTM". End with: `Review by Claude Code`

### If local mode: output the review as text

Do NOT post anything to GitHub. Instead, output the review directly as text.

For each issue found, output:

```text
**<file_path>:<line_number>** — <what's wrong and how to fix it>
```

When the fix is a concrete line change, include a fenced code block showing the suggested replacement.

At the end, output a summary section listing all issues. If no issues: just say "LGTM".

End with: `Review by Claude Code`

## What NOT to do

- Do not comment on markdown formatting preferences
- Do not suggest refactors unrelated to the PR's purpose
- Do not praise code — only flag problems or stay silent
- If no issues are found, post only a summary saying "LGTM"
- Do not flag style issues already covered by golangci-lint (errcheck, govet, ineffassign, staticcheck, unused) or pre-commit hooks (gofmt, goimports, gofumpt, codespell, markdownlint)
