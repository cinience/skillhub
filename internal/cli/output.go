package cli

import (
	"fmt"
	"os"
	"strings"
)

// Colors for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "%sError:%s %s\n", colorRed, colorReset, msg)
}

func printSuccess(msg string) {
	fmt.Printf("%s%s%s\n", colorGreen, msg, colorReset)
}

func printHeader(msg string) {
	fmt.Printf("%s%s%s%s\n", colorBold, colorCyan, msg, colorReset)
}

func printField(label, value string) {
	fmt.Printf("  %s%-16s%s %s\n", colorDim, label+":", colorReset, value)
}

// printTable prints a simple aligned table. cols defines column headers and widths.
func printTable(headers []string, widths []int, rows [][]string) {
	// Print header
	var hdr strings.Builder
	for i, h := range headers {
		if i < len(widths) {
			hdr.WriteString(fmt.Sprintf("%-*s", widths[i], h))
		} else {
			hdr.WriteString(h)
		}
		if i < len(headers)-1 {
			hdr.WriteString("  ")
		}
	}
	fmt.Printf("%s%s%s\n", colorBold, hdr.String(), colorReset)

	// Print separator
	var sep strings.Builder
	for i, w := range widths {
		sep.WriteString(strings.Repeat("-", w))
		if i < len(widths)-1 {
			sep.WriteString("  ")
		}
	}
	fmt.Printf("%s%s%s\n", colorDim, sep.String(), colorReset)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				// Truncate if too long
				if len(cell) > widths[i] {
					cell = cell[:widths[i]-1] + "…"
				}
				fmt.Printf("%-*s", widths[i], cell)
			} else {
				fmt.Print(cell)
			}
			if i < len(row)-1 {
				fmt.Print("  ")
			}
		}
		fmt.Println()
	}
}

func exitWithError(msg string) {
	printError(msg)
	os.Exit(1)
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getNum(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		f, ok := v.(float64)
		if ok {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%v", v)
	}
	return "0"
}
