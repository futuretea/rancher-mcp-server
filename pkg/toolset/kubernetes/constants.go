package kubernetes

const (
	// Resource conversion constants
	MilliCPUBase = 1000
	BytesPerKi   = 1024
	BytesPerMi   = 1024 * 1024
	BytesPerGi   = 1024 * 1024 * 1024
	BytesPerTi   = 1024 * 1024 * 1024 * 1024

	// Decimal memory units (1000-based)
	DecimalKilo = 1000
	DecimalMega = 1000 * 1000
	DecimalGiga = 1000 * 1000 * 1000

	// Log parsing constants
	RFC3339NanoLen = 30 // Length of RFC3339Nano timestamp string
	RFC3339Len     = 20 // Length of RFC3339 timestamp string (without nanoseconds)

	// Default log tail lines
	DefaultTailLines    = 100
	PodInspectTailLines = 50

	// Table formatting constants
	DefaultNameTruncateLen = 40
	DefaultNSTruncateLen   = 20
	DefaultKindTruncateLen = 15

	// Pagination defaults
	DefaultLimit = 100
	DefaultPage  = 1

	// Watch/diff defaults
	DefaultIntervalSeconds = 10
	DefaultIterations      = 6
	MinIntervalSeconds     = 1
	MaxIntervalSeconds     = 600
	MinIterations          = 1
	MaxIterations          = 100

	// Dep graph defaults
	DefaultMaxDepth = 10
	MinDepth        = 1
	MaxDepth        = 20
)
