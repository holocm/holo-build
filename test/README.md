# Test suite for `holo-build`

There are two types of tests here:

* [Compiler tests](./compiler) check how a certain package description is
  parsed, and compiled by the various package generators.
* [Interface tests](./interface) check how holo-build behaves under a certain
  set of command-line switches or in a certain environment.

To run the tests, use the make targets `test` or `check` in the top-level directory.
