// Package buildinfo holds the version embedded in release binaries.
package buildinfo

var Version = "dev"

func Current() string {
	if Version == "" {
		return "dev"
	}
	return Version
}
