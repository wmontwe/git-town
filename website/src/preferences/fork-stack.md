# Fork stack

The `fork-stack` setting creates fork-compatible stacked pull requests on
GitHub.
This is useful when the development remote points to a contributor fork
and the pull requests must be created in the upstream repository.

```toml
[propose]
fork-stack = true
fork-stack-label = "stacked-change"
```

The default for `fork-stack` is `false`.

## Label stacked pull requests

Set `fork-stack-label` to the name of an existing label in the upstream GitHub
repository.
Git Town adds this label to every non-root pull request when creating
or updating the stack.
It does not add the label to the stack root because
that pull request does not depend on another pull request in the stack.
The default is empty, which disables labeling.

When `fork-stack` is enabled,
[`git town propose`](../commands/propose.md) creates or updates pull requests
for the current branch and all its ancestor branches.
It does not automatically propose unfinished descendant branches.
Use `git town propose --stack` to also propose review-ready descendants.
Git Town skips prototype branches other than the current branch
and their subtrees so it does not publish unfinished work implicitly.
If the current branch is a prototype,
Git Town converts and proposes only that prototype.
If the current branch depends on a prototype ancestor,
Git Town aborts and asks you to convert the ancestor explicitly.
Selected parked branches become feature branches and are proposed.

Git Town pushes the selected branches to the configured
[development remote](dev-remote.md),
discovers that fork's upstream repository,
and creates or updates their pull requests against the upstream default branch.
Each pull request is cumulative because GitHub cannot use a branch from a fork
as its base.

Git Town maintains a section in each pull request description
that lists all pull requests in the stack and records the logical parent.
For every pull request after the stack root,
the section also links to each commit to review
and explains why the pull request includes earlier stacked changes.
The root pull request does not need these details
because GitHub's normal pull request diff shows only its changes.

Content outside the managed section remains unchanged
when Git Town updates an existing pull request.
When creating a pull request,
Git Town uses the upstream repository's default `pull_request_template.md` from
`.github`, the repository root, or `docs`.
A body supplied via `--body` or `--body-file` overrides this template
for the currently checked-out branch.

After creating or updating the stack,
Git Town prints the URLs of all its pull requests in stack order.

Fork-stack mode requires `github.com`, the `gh` executable,
and an authenticated GitHub CLI session.
The development repository
and upstream repository must belong to the same GitHub fork network.

No native GitHub Stack object is created.
Merge pull requests bottom-first.
When fork-stack mode is enabled,
`git town sync` refreshes managed review metadata after rebasing branches.
If the previous root was merged,
the first remaining pull request becomes the new root and loses the stack label
and cumulative-review details automatically.
Rerun `git town propose` after changing which branches belong to the stack.
