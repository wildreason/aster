package main

// The aliases bridge: Phase 1 moved the render pipeline into engine/ and this
// file re-exports it under the unqualified names the CLI has always used, so
// the extraction produces a near-zero diff in CLI files (golden-parity's best
// friend). New CLI code may use engine.X directly; existing code stays put.

import "github.com/wildreason/aster/engine"

// Types
type (
	Block            = engine.Block
	BlockIndex       = engine.BlockIndex
	BlockContentType = engine.BlockContentType
	Parser           = engine.Parser
	FileParser       = engine.FileParser
	Frontmatter      = engine.Frontmatter
	DocMeta          = engine.DocMeta
	ConversationTurn = engine.ConversationTurn
	TurnPart         = engine.TurnPart
	MarkdownParser   = engine.MarkdownParser
	HTMLParser       = engine.HTMLParser
	JSONLParser      = engine.JSONLParser
	DiffParser       = engine.DiffParser
	CsvParser        = engine.CsvParser
	ImageParser      = engine.ImageParser
	VideoParser      = engine.VideoParser
	TxtParser        = engine.TxtParser
	TodoParser       = engine.TodoParser
	ContractParser   = engine.ContractParser
	fileType         = engine.FileType
)

// Content-type constants
const (
	BlockContentPlain      = engine.BlockContentPlain
	BlockContentJSON       = engine.BlockContentJSON
	BlockContentYAML       = engine.BlockContentYAML
	BlockContentDiff       = engine.BlockContentDiff
	BlockContentShell      = engine.BlockContentShell
	BlockContentTranscript = engine.BlockContentTranscript
	BlockContentImage      = engine.BlockContentImage
	BlockContentVideo      = engine.BlockContentVideo
)

// Functions and shared data
var (
	fileTypes               = engine.FileTypes
	detectFileType          = engine.DetectFileType
	detectParser            = engine.DetectParser
	detectParserFromContent = engine.DetectParserFromContent
	RenderStaticHTMLPage    = engine.RenderStaticHTMLPage
	RenderHTMLPage          = engine.RenderHTMLPage
	RenderIndexPage         = engine.RenderIndexPage
	ParseFrontmatter        = engine.ParseFrontmatter
	HTMLToMarkdown          = engine.HTMLToMarkdown
	ScanContentTypes        = engine.ScanContentTypes
	ParseHunks              = engine.ParseHunks
	NewDiffFormatter        = engine.NewDiffFormatter
	NewBlockIndex           = engine.NewBlockIndex
	GetFileFromDiff         = engine.GetFileFromDiff
	hasStructuredPatch      = engine.HasStructuredPatch
	extractStructuredPatch  = engine.ExtractStructuredPatch
	isTableLine             = engine.IsTableLine
	isTableSeparator        = engine.IsTableSeparator
	parseTableCells         = engine.ParseTableCells
	imageMIME               = engine.ImageMIME
	videoMIME               = engine.VideoMIME
	imageExtensions         = engine.ImageExtensions
	videoExtensions         = engine.VideoExtensions
)

// Typed Block.Data payloads referenced by CLI surfaces
type (
	ImageData      = engine.ImageData
	VideoData      = engine.VideoData
	CsvData        = engine.CsvData
	TranscriptData = engine.TranscriptData
	ContractData   = engine.ContractData
)
