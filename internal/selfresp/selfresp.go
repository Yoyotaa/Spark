// Package selfresp makes Spark the "responsible process" for its own macOS
// privacy permissions.
//
// macOS (TCC) attributes the permission requests of a process launched from
// a terminal to the app hosting that terminal: started from GoLand, Spark
// asks for the microphone *as GoLand*. The dialog then belongs to GoLand,
// and speech recognition cannot even be asked since GoLand does not declare
// it. Relaunching with the responsibility disclaimed (as Chrome or VS Code
// do for their helpers) makes the requests come from Spark itself, with its
// own Info.plist and its own entries in System Settings.
package selfresp

const envDisclaimed = "SPARK_DISCLAIMED"
