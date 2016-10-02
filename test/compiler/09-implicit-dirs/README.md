The new `FSNode`-based package representation had a problem with
implicitly-created directories conflicting with explicit directory entries in
the `input.toml`. Consider, for example, the following situation:

1. The parser encounters a `[[directory]]` with `path = "/etc/foo/bar". In the
   filesystem tree for the package, directories `/etc` and `/etc/foo` will be
   created implicitly in order to be able to insert the directory `/etc/foo/bar`.

2. Next, the parser sees a `[[directory]]` with `path = "/etc/foo"`. Since such
   a directory already exists in the tree, a "duplicate entry" error is thrown,
   even though the user did not specify this entry multiple times.

To solve this, the implementation marks implicitly-created directories, and
silently replaces these by explicit directory entries.
