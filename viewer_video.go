package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)


func viewVideo(path string) {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Try ffprobe for metadata
	if ffprobePath, err := exec.LookPath("ffprobe"); err == nil {
		cmd := exec.Command(ffprobePath,
			"-v", "quiet",
			"-print_format", "flat",
			"-show_format",
			"-show_streams",
			path,
		)
		output, err := cmd.Output()
		if err == nil {
			fmt.Printf("Video: %s\n", filepath.Base(path))
			fmt.Printf("Size: %d bytes\n", info.Size())
			// Parse key fields from flat output
			for _, line := range strings.Split(string(output), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "streams.stream.0.width=") {
					fmt.Printf("Width: %s\n", trimFFVal(line))
				} else if strings.HasPrefix(line, "streams.stream.0.height=") {
					fmt.Printf("Height: %s\n", trimFFVal(line))
				} else if strings.HasPrefix(line, "streams.stream.0.codec_name=") {
					fmt.Printf("Codec: %s\n", trimFFVal(line))
				} else if strings.HasPrefix(line, "format.duration=") {
					fmt.Printf("Duration: %ss\n", trimFFVal(line))
				}
			}
			fmt.Println()
		}
	}

	// Try ffplay for playback
	if ffplayPath, err := exec.LookPath("ffplay"); err == nil {
		cmd := exec.Command(ffplayPath, "-autoexit", "-loglevel", "quiet", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return
		}
	}

	// Fallback: show file info
	fmt.Printf("Video: %s\n", path)
	fmt.Printf("Size: %d bytes\n", info.Size())
	fmt.Printf("Extension: %s\n", filepath.Ext(path))
	fmt.Println("\nInstall ffmpeg for terminal video playback:")
	fmt.Println("  brew install ffmpeg")
}

// trimFFVal extracts the value from a ffprobe flat-format line like key="value"
func trimFFVal(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.Trim(parts[1], "\"")
}
