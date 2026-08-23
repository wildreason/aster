package main


// Command represents a parsed user command
type Command struct {
	Action string // next, prev, quit
	Arg    string
}

// Navigator manages navigation state
type Navigator struct {
	index       *BlockIndex
	currentPos  int // Position in block list
	currentPage int // Current page within block (0-indexed)
	history     []int
	maxHistory  int
}

// NewNavigator creates a new navigator
func NewNavigator(index *BlockIndex) *Navigator {
	return &Navigator{
		index:       index,
		currentPos:  0,
		currentPage: 0,
		history:     []int{},
		maxHistory:  10,
	}
}

// ExecuteCommand processes a command and returns the result
func (nav *Navigator) ExecuteCommand(cmd *Command) (string, *Block, bool) {
	if cmd == nil {
		return "", nil, false
	}

	switch cmd.Action {
	case "next":
		return nav.handleNext()
	case "prev":
		return nav.handlePrev()
	case "quit", "exit":
		return "", nil, true
	default:
		return "", nil, false
	}
}

// handleNext jumps to the next block
func (nav *Navigator) handleNext() (string, *Block, bool) {
	if nav.currentPos+1 >= len(nav.index.Blocks) {
		return "Already at the last block.", nil, false
	}

	nav.saveHistory(nav.currentPos)
	nav.currentPos++
	nav.currentPage = 0
	block := nav.index.GetBlockByPosition(nav.currentPos)

	return "", block, false
}

// handlePrev jumps to the previous block
func (nav *Navigator) handlePrev() (string, *Block, bool) {
	if nav.currentPos <= 0 {
		return "Already at the first block.", nil, false
	}

	nav.saveHistory(nav.currentPos)
	nav.currentPos--
	nav.currentPage = 0
	block := nav.index.GetBlockByPosition(nav.currentPos)

	return "", block, false
}

// saveHistory saves current position to history
func (nav *Navigator) saveHistory(pos int) {
	nav.history = append(nav.history, pos)
	if len(nav.history) > nav.maxHistory {
		nav.history = nav.history[1:]
	}
}

// GetCurrentBlock returns the current block
func (nav *Navigator) GetCurrentBlock() *Block {
	return nav.index.GetBlockByPosition(nav.currentPos)
}

// GetCurrentPosition returns current position in document
func (nav *Navigator) GetCurrentPosition() int {
	return nav.currentPos
}

// GetTotalBlocks returns total number of blocks
func (nav *Navigator) GetTotalBlocks() int {
	return len(nav.index.Blocks)
}

// GetCurrentPage returns the current page (0-indexed)
func (nav *Navigator) GetCurrentPage() int {
	return nav.currentPage
}

// NextPage moves to the next page within current block
func (nav *Navigator) NextPage() bool {
	block := nav.GetCurrentBlock()
	if block == nil {
		return false
	}

	if nav.currentPage+1 >= block.TotalPages {
		return false
	}

	nav.currentPage++
	return true
}

// PrevPage moves to the previous page within current block
func (nav *Navigator) PrevPage() bool {
	if nav.currentPage <= 0 {
		return false
	}

	nav.currentPage--
	return true
}
