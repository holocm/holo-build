# v1.1.1 (2020-10-09)

Bugfixes:

- Fix the version format for Pacman packages to ensure that prerelease packages
  (e.g. "1.2.3-beta.4") are correctly identified as being older than their
  corresponding final releases (e.g. "1.2.3").

# v1.1.0 (2020-10-06)

New features:

- The fields `PrereleaseType` and `PrereleaseVersion` were added to type
  Package.

# v1.0.0 (2018-12-20)

Initial standalone release. This code originates in [holo-build](https://github.com/holocm/holo-build).
