package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"golang.org/x/term"
)


func imgTermWidth() string {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			return strconv.Itoa(w)
		}
	}
	return "80"
}

func imgDetectFormat() string {
	tp := os.Getenv("TERM_PROGRAM")
	switch tp {
	case "iTerm.app", "WezTerm", "Hyper":
		return "iterm"
	case "kitty":
		return "kitty"
	}
	return "symbols"
}

func viewImage(path string) {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Try chafa
	if chafaPath, err := exec.LookPath("chafa"); err == nil {
		format := imgDetectFormat()
		w := imgTermWidth()
		cmd := exec.Command(chafaPath, "--size="+w, "--format="+format, path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			fmt.Printf("\n%s (%d bytes)\n", filepath.Base(path), info.Size())
			return
		}
		// Fallback to symbols
		if format != "symbols" {
			cmd = exec.Command(chafaPath, "--size="+w, "--format=symbols", path)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				fmt.Printf("\n%s (%d bytes)\n", filepath.Base(path), info.Size())
				return
			}
		}
	}

	// Try imgcat (iTerm2 native)
	if imgcatPath, err := exec.LookPath("imgcat"); err == nil {
		cmd := exec.Command(imgcatPath, path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return
		}
	}

	// Fallback
	fmt.Printf("Image: %s\n", path)
	fmt.Printf("Size: %d bytes\n", info.Size())
	fmt.Printf("Extension: %s\n", filepath.Ext(path))
	fmt.Println("\nInstall chafa for terminal image preview:")
	fmt.Println("  brew install chafa")
}
