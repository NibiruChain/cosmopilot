package apps

import (
	"os"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
)

// currentArch returns the current CPU architecture in Docker/OCI format
func currentArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// supportsArch checks if the app supports the given architecture
func (t TestApp) supportsArch(arch string) bool {
	// If no architectures specified, assume all are supported
	if len(t.Architectures) == 0 {
		return true
	}
	for _, a := range t.Architectures {
		if a == arch {
			return true
		}
	}
	return false
}

// allApps returns all available test applications
func allApps() []TestApp {
	return []TestApp{
		Nibiru(),
		Osmosis(),
		CosmosHub(),
	}
}

// All returns all test applications, filtered by:
// 1. Current CPU architecture (apps must support the current arch)
// 2. TEST_APPS environment variable (optional, comma-separated list of app names)
// Example: TEST_APPS=nibiru,osmosis
func All() []TestApp {
	arch := currentArch()

	// Start with apps that support the current architecture
	var archFiltered []TestApp
	for _, app := range allApps() {
		if app.supportsArch(arch) {
			archFiltered = append(archFiltered, app)
		}
	}

	// If TEST_APPS is not set, return arch-filtered apps
	filter := os.Getenv("TEST_APPS")
	if filter == "" {
		return archFiltered
	}

	// Parse filter into a set of lowercase names
	filterSet := make(map[string]bool)
	for _, name := range strings.Split(filter, ",") {
		filterSet[strings.ToLower(strings.TrimSpace(name))] = true
	}

	// Filter apps by name
	var filtered []TestApp
	for _, app := range archFiltered {
		if filterSet[strings.ToLower(app.Name)] {
			filtered = append(filtered, app)
		}
	}

	return filtered
}

// ForEachApp runs a test function for each registered app using Ginkgo's DescribeTable.
// The description is the test description, and fn is the test function to run for each app.
func ForEachApp(description string, fn func(TestApp)) {
	args := []interface{}{fn}
	for _, app := range All() {
		args = append(args, Entry(app.Name, app))
	}
	DescribeTable(description, args...)
}
