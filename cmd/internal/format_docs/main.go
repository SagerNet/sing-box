package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/log"
)

func main() {
	err := filepath.Walk("docs", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		return processFile(path)
	})
	if err != nil {
		log.Fatal(err)
	}
}

func processFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	modified := false
	result := make([]string, 0, len(lines))

	inQuoteBlock := false
	materialLines := []int{} // indices of :material- lines in the block

	for _, line := range lines {
		// Check for quote block start
		if strings.HasPrefix(line, "!!! quote \"") && strings.Contains(line, "sing-box") {
			inQuoteBlock = true
			materialLines = nil
			result = append(result, line)
			continue
		}

		// Inside a quote block
		if inQuoteBlock {
			trimmed := strings.TrimPrefix(line, "    ")
			isMaterialLine := strings.HasPrefix(trimmed, ":material-")
			isEmpty := strings.TrimSpace(line) == ""
			isIndented := strings.HasPrefix(line, "    ")

			if isMaterialLine {
				materialLines = append(materialLines, len(result))
				result = append(result, line)
				continue
			}

			// Block ends when:
			// - Empty line AFTER we've seen material lines, OR
			// - Non-indented, non-empty line
			blockEnds := (isEmpty && len(materialLines) > 0) || (!isEmpty && !isIndented)
			if blockEnds {
				// Process collected material lines
				if len(materialLines) > 0 {
					for j, idx := range materialLines {
						isLast := j == len(materialLines)-1
						resultLine := strings.TrimRight(result[idx], " ")
						if !isLast {
							// Add trailing two spaces for non-last lines
							resultLine += "  "
						}
						if result[idx] != resultLine {
							modified = true
							result[idx] = resultLine
						}
					}
				}
				inQuoteBlock = false
				materialLines = nil
			}
		}

		result = append(result, line)
	}

	// Handle case where file ends while still in a block
	if inQuoteBlock && len(materialLines) > 0 {
		for j, idx := range materialLines {
			isLast := j == len(materialLines)-1
			resultLine := strings.TrimRight(result[idx], " ")
			if !isLast {
				resultLine += "  "
			}
			if result[idx] != resultLine {
				modified = true
				result[idx] = resultLine
			}
		}
	}

	if modified {
		newContent := strings.Join(result, "\n")
		if !bytes.Equal(content, []byte(newContent)) {
			log.Info("formatted: ", path)
			return os.WriteFile(path, []byte(newContent), 0o644)
		}
	}

	return nil
}
