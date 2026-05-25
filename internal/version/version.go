package version

import "strings"

// BuildVersion is set at link-time using -ldflags.
var BuildVersion = "0.1.0-alpha"

// ProductName is the public product codename.
const ProductName = "AstraLink"

// GetVersion returns the current build version.
func GetVersion() string {
	v := strings.TrimSpace(BuildVersion)
	if v == "" {
		return "dev"
	}
	return v
}
