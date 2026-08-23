package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// watchFile monitors a JSONL file for new content and parses new blocks
func watchFile(filePath string, jsonlParser *JSONLParser, index *BlockIndex, navigator *Navigator, onNewBlock func(), stopCh <-chan struct{}) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	// Seek to end of file
	offset, err := file.Seek(0, 2)
	if err != nil {
		return
	}

	buf := make([]byte, 4096)
	var partial string
	turnNumber := len(index.Blocks)
	var currentTurn *ConversationTurn
	var currentBlockIdx int = -1

	showUser := jsonlParser.Filters["user"]
	showAssistant := jsonlParser.Filters["assistant"]
	showDiff := jsonlParser.Filters["diff"]
	showToolResult := jsonlParser.Filters["tool_result"]

	rebuildCurrentBlock := func() {
		if currentTurn == nil || currentBlockIdx < 0 {
			return
		}
		newBlock := jsonlParser.CreateTurnBlock(currentTurn, turnNumber)
		index.Blocks[currentBlockIdx] = newBlock
		index.NameIndex[strings.ToLower(newBlock.Name)] = currentBlockIdx
		onNewBlock()
	}

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		time.Sleep(500 * time.Millisecond)

		n, err := file.Read(buf)
		if err != nil && err.Error() != "EOF" {
			continue
		}
		if n == 0 {
			stat, _ := file.Stat()
			if stat != nil && stat.Size() < offset {
				file.Seek(0, 0)
				offset = 0
			}
			continue
		}

		offset += int64(n)
		data := partial + string(buf[:n])
		lines := strings.Split(data, "\n")

		partial = lines[len(lines)-1]
		lines = lines[:len(lines)-1]

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if jsonlParser == nil {
				continue
			}

			msg, msgType, isToolResult := jsonlParser.ParseLineInfo(line)
			if msg == nil {
				continue
			}

			// Tool results: add diffs and/or tool output to current turn
			if msgType == "user" && isToolResult {
				if currentTurn != nil {
					needsRebuild := false

					if showDiff && hasStructuredPatch(msg) {
						diffContent := extractStructuredPatch(msg)
						if diffContent != "" {
							toolUseResult, _ := msg["toolUseResult"].(map[string]interface{})
							fp, _ := toolUseResult["filePath"].(string)
							currentTurn.Parts = append(currentTurn.Parts, TurnPart{
								Type:    "diff",
								Content: diffContent,
								Meta:    fp,
							})
							needsRebuild = true
						}
					}

					if showToolResult {
						toolContent := jsonlParser.ExtractToolResultContent(msg)
						if toolContent != "" {
							currentTurn.Parts = append(currentTurn.Parts, TurnPart{
								Type:    "tool_result",
								Content: toolContent,
							})
							needsRebuild = true
						}
					}

					if needsRebuild {
						rebuildCurrentBlock()
					}
				}
				continue
			}

			// User message: start a new turn
			if msgType == "user" && showUser {
				userContent := jsonlParser.ExtractUserContent(msg)
				if userContent != "" {
					turnNumber++
					currentTurn = &ConversationTurn{
						Parts:   []TurnPart{{Type: "user", Content: userContent}},
						LineNum: 0,
					}
					newBlock := jsonlParser.CreateTurnBlock(currentTurn, turnNumber)
					index.Blocks = append(index.Blocks, newBlock)
					currentBlockIdx = len(index.Blocks) - 1
					index.NameIndex[strings.ToLower(newBlock.Name)] = currentBlockIdx
					onNewBlock()
				}
				continue
			}

			// Assistant message: add to current turn
			if msgType == "assistant" && showAssistant && currentTurn != nil {
				assistantContent := jsonlParser.ExtractAssistantContent(msg)
				if assistantContent != "" {
					currentTurn.Parts = append(currentTurn.Parts, TurnPart{
						Type:    "assistant",
						Content: assistantContent,
					})
					rebuildCurrentBlock()
				}
			}
		}
	}
}

// watchGenericFile monitors any file for changes and reloads it
func watchGenericFile(filePath string, onReload func([]Block), stopCh <-chan struct{}) {
	parser := detectParser(filePath)
	var lastModTime time.Time

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		time.Sleep(500 * time.Millisecond)

		stat, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		if stat.ModTime().After(lastModTime) {
			lastModTime = stat.ModTime()

			content, err := ioutil.ReadFile(filePath)
			if err != nil {
				continue
			}

			blocks := parser.Parse(string(content))
			if len(blocks) > 0 {
				onReload(blocks)
			}
		}
	}
}

// runFollowMode runs the follow mode TUI
func runFollowMode(filePath string, fileContent string, isJSONL bool, termWidth int, style string, borderStyle BorderStyle) {
	var blocks []Block
	var index *BlockIndex
	var jsonlParser *JSONLParser

	// Parse initial blocks
	if isJSONL {
		jsonlParser = &JSONLParser{Decorate: tuiDecoratePart}
		filters := showContentSelector(fileContent)
		jsonlParser.Filters = filters
		blocks = jsonlParser.Parse(fileContent)
	} else {
		parser := detectParser(filePath)
		blocks = parser.Parse(fileContent)
	}

	if len(blocks) == 0 {
		fmt.Println("Error: No blocks found in file.")
		return
	}

	index = NewBlockIndex(blocks)
	navigator := NewNavigator(index)

	// Start TUI
	app := tview.NewApplication()
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	textView.SetBorderPadding(0, 0, 2, 2)

	// Start at last block (follow mode shows latest)
	navigator.currentPos = len(index.Blocks) - 1
	currentBlock := navigator.GetCurrentBlock()
	if currentBlock != nil {
		navigator.currentPage = currentBlock.TotalPages - 1
		rendered := FormatBlockPage(currentBlock, navigator.GetCurrentPage(), termWidth, borderStyle)
		textView.SetText(tview.TranslateANSI(rendered))
	}

	// File watcher
	fileWatcherStop := make(chan struct{})

	onNewBlock := func() {
		app.QueueUpdateDraw(func() {
			navigator.currentPos = len(index.Blocks) - 1
			currentBlock := navigator.GetCurrentBlock()
			if currentBlock != nil {
				navigator.currentPage = currentBlock.TotalPages - 1
				rendered := FormatBlockPage(currentBlock, navigator.GetCurrentPage(), termWidth, borderStyle)
				textView.SetText(tview.TranslateANSI(rendered))
			}
		})
	}

	if filePath != "" && filePath != "stdin" {
		if isJSONL {
			go watchFile(filePath, jsonlParser, index, navigator, onNewBlock, fileWatcherStop)
		} else {
			go watchGenericFile(filePath, func(newBlocks []Block) {
				app.QueueUpdateDraw(func() {
					index.Blocks = newBlocks
					index.NameIndex = make(map[string]int)
					for i, b := range newBlocks {
						index.NameIndex[strings.ToLower(b.Name)] = i
					}
					navigator.currentPos = len(newBlocks) - 1
					navigator.currentPage = 0
					currentBlock := navigator.GetCurrentBlock()
					if currentBlock != nil {
						rendered := FormatBlockPlain(currentBlock, termWidth, style, borderStyle)
						textView.SetText(tview.TranslateANSI(rendered))
					}
				})
			}, fileWatcherStop)
		}
	}

	// Key bindings
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			app.Stop()
			return nil
		case 'j':
			navigator.ExecuteCommand(&Command{Action: "next"})
			navigator.currentPage = 0
			currentBlock := navigator.GetCurrentBlock()
			if currentBlock != nil {
				rendered := FormatBlockPage(currentBlock, navigator.GetCurrentPage(), termWidth, borderStyle)
				textView.SetText(tview.TranslateANSI(rendered))
				textView.ScrollToBeginning()
			}
			return nil
		case 'J':
			navigator.ExecuteCommand(&Command{Action: "prev"})
			navigator.currentPage = 0
			currentBlock := navigator.GetCurrentBlock()
			if currentBlock != nil {
				rendered := FormatBlockPage(currentBlock, navigator.GetCurrentPage(), termWidth, borderStyle)
				textView.SetText(tview.TranslateANSI(rendered))
				textView.ScrollToBeginning()
			}
			return nil
		}

		switch event.Key() {
		case tcell.KeyDown, tcell.KeyUp:
			return event
		case tcell.KeyRight:
			navigator.ExecuteCommand(&Command{Action: "next"})
			navigator.currentPage = 0
			currentBlock := navigator.GetCurrentBlock()
			if currentBlock != nil {
				rendered := FormatBlockPage(currentBlock, navigator.GetCurrentPage(), termWidth, borderStyle)
				textView.SetText(tview.TranslateANSI(rendered))
				textView.ScrollToBeginning()
			}
			return nil
		case tcell.KeyLeft:
			navigator.ExecuteCommand(&Command{Action: "prev"})
			navigator.currentPage = 0
			currentBlock := navigator.GetCurrentBlock()
			if currentBlock != nil {
				rendered := FormatBlockPage(currentBlock, navigator.GetCurrentPage(), termWidth, borderStyle)
				textView.SetText(tview.TranslateANSI(rendered))
				textView.ScrollToBeginning()
			}
			return nil
		}

		return event
	})

	if err := app.SetRoot(textView, true).Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
