// Package kit is the shared base every Rhythm tool imports:
// branding, output, config and self-update live here.
package kit

import "fmt"

// Banner returns the standard header line for a tool.
func Banner(tool, version string) string {
	return fmt.Sprintf("♪ %s  %s", tool, version)
}
