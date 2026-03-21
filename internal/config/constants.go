package config

// Collector names
const (
	CollectorHTMLVersion = "htmlversion"
	CollectorTitle       = "title"
	CollectorHeadings    = "headings"
	CollectorLoginForm   = "loginform"
	CollectorLinks       = "links"
)

// DefaultCollectors is the default list of collectors to run
var DefaultCollectors = []string{
	CollectorHTMLVersion,
	CollectorTitle,
	CollectorHeadings,
	CollectorLoginForm,
	CollectorLinks,
}

// Cache modes
const (
	CacheModeMemory   = "memory"
	CacheModeRedis    = "redis"
	CacheModeMulti    = "multi"
	CacheModeDisabled = "disabled"
)

// Link checking modes
const (
	LinkCheckModeSync     = "sync"
	LinkCheckModeAsync    = "async"
	LinkCheckModeHybrid   = "hybrid"
	LinkCheckModeDisabled = "disabled"
)

// Log levels
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Log formats
const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)
