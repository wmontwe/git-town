# Fork-stack proposal specification

## Purpose

Fork-stack mode supports stacked pull requests
when contributors push branches to a GitHub fork
but open pull requests in the upstream repository.
GitHub does not allow an upstream pull request to use a branch in a contributor
fork as its base.
Git Town therefore creates cumulative pull requests against the upstream default
branch and records the logical stack relationship in their descriptions.

## Terminology

- **Development fork:** the repository configured by `hosting.dev-remote`.
- **Upstream repository:** the source repository of the development fork.
- **Logical parent:** the Git Town parent branch of a stacked branch.
- **Stack root:** the first branch in a stack
  that still has an open pull request.
  It has no other open pull request below it.
- **Managed section:** the part of a pull request description between the
  `git-town-fork-stack:start` and `git-town-fork-stack:end` markers.

## Configuration

```toml
[propose]
fork-stack = true
fork-stack-label = "stacked-change"
```

- `fork-stack` defaults to `false`.
- `fork-stack-label` is optional and defaults to empty.
- The configured label must already exist in the upstream GitHub repository.
- Git Town applies the label to non-root pull requests only.

## Platform requirements

Fork-stack mode:

- supports `github.com` only
- requires the `gh` executable and an authenticated GitHub CLI session
- requires the development and upstream repositories to belong to the same
  GitHub fork network
- creates no native GitHub stack object

## Propose behavior

### Branch selection

- `git town propose` creates or updates pull requests for the current branch and
  all its ancestor branches in the stack.
- It does not automatically propose unfinished descendant branches.
- `git town propose --stack` also selects review-ready branches across the
  entire connected stack tree, including sibling leaves.
- All already-published pull requests in the connected tree are available as
  display context, even when they are not selected for creation or update.
- A prototype branch other than the current branch and its entire subtree are
  excluded so unfinished work is not converted, pushed, or proposed implicitly.
- If the current branch is a prototype, only that branch becomes a feature
  branch and is proposed.
- If the current branch depends on a prototype ancestor, Git Town aborts and
  asks the user to convert that ancestor explicitly.
- Selected parked branches become feature branches and are proposed.

### Pull request discovery

For every branch in the relevant stack context,
Git Town searches the upstream repository
for an open pull request whose head is the matching branch in the development
fork.
Closed and merged pull requests are not considered part of the open stack.
Git Town reuses matching open pull requests
and creates only the selected pull requests that do not exist yet.
Unpublished context branches are omitted from the managed stack list
and are not created implicitly.

### Pull request creation

Every fork-stack pull request:

- is opened in the upstream repository
- uses the upstream default branch as its GitHub base
- uses the corresponding branch in the development fork as its head
- is cumulative from the upstream default branch

Git Town derives titles from a single commit subject when possible
and otherwise from the branch name.
An explicit `--title` applies to the currently checked-out branch.

For a new pull request, Git Town loads the upstream repository's default
`pull_request_template.md` from `.github`, the repository root, or `docs`.
An explicit `--body` or `--body-file` overrides the template
for the currently checked-out branch.
Repository templates and explicit bodies are never applied to existing pull
requests.

### Managed description section

Git Town replaces only the managed section of an existing pull request
description.
User-authored content outside that section remains unchanged.

Every managed section contains:

- links to the open ancestor pull requests, the current pull request, and its
  open descendant pull requests
- a marker recording the branch's logical parent
- an indication of which pull request is current

A pull request at a branching point
therefore shows every published descendant path and leaf.
A pull request after a split shows only its own ancestor and descendant path,
not sibling leaves.

A non-root pull request additionally contains:

- a **Commits to review** list
- one link per commit in the logical-parent-to-branch range,
  ordered oldest first
- an explanation that the pull request includes earlier stacked changes because
  GitHub cannot use a branch from a fork as its base

The stack root contains neither the commit list
nor the cumulative-change explanation
because GitHub's normal pull request diff already shows only the root change.

### Labels

When `fork-stack-label` is configured:

- Git Town adds it to every non-root pull request.
- Git Town does not add it to a root pull request.
- If merging the previous root promotes another pull request to root, Git Town
  removes the configured label from the new root.
- Repeated updates are idempotent because GitHub does not duplicate labels.

### Command output

After a successful propose operation,
Git Town prints all created or updated pull request URLs in stack order
and reminds the user to merge bottom-first.

## Sync behavior

When fork-stack mode is enabled and Git Town is online,
`git town sync` updates existing fork-stack pull requests
for the feature branches involved in the sync.
It never creates missing pull requests.

The update runs after branches have been synchronized and pushed
so commit links contain the post-rebase commit SHAs.
It:

- discovers open pull requests again
- refreshes the managed **Commits to review** section
- removes links to commits replaced by a rebase
- identifies roots based on open ancestor pull requests rather than stale local
  ancestry alone
- removes review details and the configured stack label when a pull request is
  promoted to root
- applies the configured label to remaining non-root pull requests
- preserves all pull request content outside the managed section

No second `git town propose` invocation should be necessary after a normal sync
or rebase.

## Merge workflow

Pull requests are merged bottom-first.
After a root pull request is merged,
the next open pull request becomes the new root on the next sync
or propose operation.
Changing stack membership
or lineage still requires rerunning `git town propose --stack`
so all managed stack sections can be regenerated.

## Failure behavior

Git Town stops and reports an error when it cannot:

- inspect the fork or upstream repository
- create or update a selected pull request
- read a discovered repository template
- determine commits in a logical stack change
- add or remove the configured label

Missing open pull requests are skipped during sync
because sync must not create proposals implicitly.
