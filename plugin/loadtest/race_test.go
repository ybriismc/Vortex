//go:build race

package loadtest

// raceEnabled reports whether the test binary was built with the race
// detector, which changes the build hashes and keeps plugins from loading.
const raceEnabled = true
