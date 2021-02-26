Release procedure
=================

This document describes how to release a new version of moco-agent.

Versioning
----------

Follow [semantic versioning 2.0.0][semver] to choose the new version number.

Prepare change log entries
--------------------------

Add notable changes since the last release to [CHANGELOG.md](CHANGELOG.md).
It should look like:

```markdown
(snip)
## [Unreleased]

### Added
- Implement ... (#35)

### Changed
- Fix a bug in ... (#33)

### Removed
- Deprecated `-option` is removed ... (#39)

(snip)
```

Bump version
------------

1. Determine a new version number.  Let it write `$VERSION` as `VERSION=x.y.z`.
1. Checkout `main` branch.
1. Make a branch to release, for example by `git neco dev "bump-v$VERSION"`
1. Edit `CHANGELOG.md` for the new version ([example][]).
1. Commit the change and push it.

    ```console
    $ git commit -a -m "Bump version to v$VERSION"
    $ git neco review
    ```
1. Merge this branch.
1. Checkout `main` branch.
1. Add a git tag, then push it.

    ```console
    $ git tag "v$VERSION"
    $ git push origin "v$VERSION"

Now the version is bumped up and the latest container image is uploaded to [GitHub Container Registry](https://github.com/orgs/cybozu-go/packages/container/package/moco-agent).

Publish GitHub release page
---------------------------

There is nothing to do.

After the release CI is successful, the tagged version will be automatically released at the [GitHub release page](https://github.com/cybozu-go/moco-agent/releases).


[semver]: https://semver.org/spec/v2.0.0.html
[example]: https://github.com/cybozu-go/etcdpasswd/commit/77d95384ac6c97e7f48281eaf23cb94f68867f79
