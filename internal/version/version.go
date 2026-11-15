package version

// Values are overridden at link time via -ldflags.
var (
	Version = "0.0.0-dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)
