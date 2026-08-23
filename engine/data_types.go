package engine

// ImageData holds payload for BlockContentImage blocks
type ImageData struct {
	Src    string // File path (server mode) or data URI (static mode)
	MIME   string // e.g. "image/png"
	Alt    string // Alt text (defaults to filename)
	Inline bool   // true = data URI, false = file path
}

// VideoData holds payload for BlockContentVideo blocks
type VideoData struct {
	Src    string // File path (server mode) or data URI (static mode)
	MIME   string // e.g. "video/mp4"
	Inline bool   // true = data URI, false = file path

	// Warning carries a non-fatal condition (e.g. too large to inline) for
	// the HOST to surface. The engine is a library: it never writes to
	// stderr itself.
	Warning string
}

// CsvData holds payload for BlockContentCSV blocks
type CsvData struct {
	Records [][]string // Header + data rows
}

// TranscriptData holds payload for BlockContentTranscript blocks
type TranscriptData struct {
	TurnParts []TurnPart
}

// ContractData holds payload for BlockContentContract blocks
type ContractData struct {
	Preamble  string
	Clauses   []contractClause
	Parties   string
	Effective string
}
