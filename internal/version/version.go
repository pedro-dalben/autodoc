package version

const (
	PlaywrightGoPin       = "v0.6201.1"
	PlaywrightGoMinimum   = "v0.6100.0"
	StoryboardSchemaVer   = 2
	CanonicalSkillVersion = "3"
)

// Version defaults to the last tagged release. GoReleaser overwrites it at
// build time from the git tag (see .goreleaser.yaml ldflags).
var Version = "0.1.0"

var (
	Commit = "dev"
	Date   = "unknown"
)
