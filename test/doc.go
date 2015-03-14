// Package testutil provides initalization for unit and integration tests.
//
// Configures logging, random number generators and other global state.
// Typical usage in _test.go files:
//
// import "v.io/x/ref/test"
// func TestMain(m *testing.M) {
//     test.Init()
//     os.Exit(m.Run())
// }
//
// InitForTest can be used within test functions as a safe alternative
// to v23.Init.
//
// func TestFoo(t *testing.T) {
//    ctx, shutdown := test.InitForTest()
//    defer shutdown()
//    ...
// }
package test