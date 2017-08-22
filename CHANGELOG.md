# v1.5.1 (2017-08-22)

Bugfixes:

- Fix a reproducibility bug where the generated package was not deterministic
  when dependencies on more than one Holo plugin were implied.

Changes:

- `/usr/bin/holo-build` now exits with non-zero status when no package format
  was given and automatic selection fails.

# v1.5 (2017-02-20)

Deprecation notices:

- The `--debian`, `--pacman` and `--rpm` options are deprecated. Use the new
  option `--format={debian,pacman,rpm}` instead.
- The `--stdout` option and its negation are deprecated. Use the new `--output`
  option instead.

New features:

- The invocation syntax was cleaned up. The new options `--format` and
  `--output` replace multiple other options which are now deprecated (see
  above).
- `holo-build` will complain when trying to overwrite an existing target file
  unless instructed to do so with the new `--force` option. No complaint will be
  issued if the target file is identical to what `holo-build` would've written.
- The much-outdated shell completion widgets were updated and now implement the
  full invocation syntax, except for deprecated options.

# v1.4 (2016-09-13)

New features:

- The new `package.architecture` field can be used to build packages
  with compiled binaries for selected x86 and ARM architectures.

Miscellaneous:

- Fix unit test failure in Go 1.7.

# v1.3 (2016-06-07)

Deprecation notices:

- The `package.setupScript` and `package.cleanupScript` keys are deprecated.
  Use the new `[[action]]` syntax instead.
- The `package.definitionFile` key is deprecated, without replacement. See
  below.
- The `--(no-)reproducible` options are deprecated. All packages produced by
  holo-build are now fully reproducible.

New features:

- Package definitions can now contain `[[action]]` sections. This syntax
  replaces `package.setupScript` and `package.cleanupScript`.

Changes:

- `package.definitionFile` is not needed anymore (and deprecated, see above).
  The definition file's name is now derived from the package name by default.
- Packages are now always built fully reproducibly, so the `--reproducible`
  option is not needed anymore (and deprecated, see above).

Miscellaneous:

- Strip binaries during build.

# v1.2 (2016-02-25)

New features:

- Add RPM generator. RPMs will now be built automatically when running on RHEL,
  Fedora, SLES, openSUSE or Mageia. RPM support is considered experimental
  because of the abysmal state of documentation for the RPM package format.

Bugfixes:

- Fix version detection when building an unpacked release tarball.

Miscellaneous:

- Use golangvend to simplify the management of library dependencies.
- Share validation logic between generators.

# v1.1.1 (2016-01-30)

Bugfixes:

- Fix a bug which caused symlinks in packages to be unpacked as empty regular
  files.

# v1.1 (2016-01-30)

Changes:

- Packages are now built in memory, without constructing a temporary directory
  tree in the filesystem. This also means that the fakeroot dependency is gone.

Bugfixes:

- Fix a syntax error in the zsh autocompletion.
- Fix various small errors in the manpage.

# v1.0 (2015-12-04)

No changes compared to the beta release.

# v1.0-beta.1 (2015-12-03)

Changes:

- holo-build is now informed about the new plugin structure of Holo. When a
  package includes files below /usr/share/holo, it will now add automatic
  dependencies on the Holo plugins used, rather than on Holo itself. (The
  dependency on Holo is expected to be implied by the plugin package.)

