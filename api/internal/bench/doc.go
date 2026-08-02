// Package bench holds the load benchmarks for Bodhveda's send paths.
//
// It carries no production code — the benchmarks live in _test.go files here so
// that they can wire real repositories and real job processors together against
// a live Postgres without any of that harness leaking into the packages under
// test. See README.md in this directory for how to run them and for the last
// recorded results.
package bench
