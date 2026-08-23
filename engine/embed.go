package engine

import _ "embed"

//go:embed embed/highlight.min.js
var highlightJS string

//go:embed embed/github.min.css
var highlightCSS string
