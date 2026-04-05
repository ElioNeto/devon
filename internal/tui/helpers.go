package tui

import (
	"fmt"
	"strings"
)

// Trunc truncates a string to max runes, appending "…" if truncated.
func Trunc(s string, max int) string {
	ru := []rune(s)
	if len(ru) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(ru[:max-1]) + "…"
}

// fmtShort formats an integer with K/M suffix for compact display.
func fmtShort(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatShort is the exported alias used by messages.go.
// (chart.go provides the package-level formatShort → fmtShort bridge,
//  but fmtShort must exist here.)

// HorzBar renders a single labelled horizontal bar.
//
//	label   – left-aligned label
//	value   – current value
//	maxV    – maximum value (used to scale the bar)
//	width   – total character width available for the bar segment
//	labelW  – fixed width reserved for the label column
func HorzBar(label string, value, maxV, width, labelW int) string {
	if maxV <= 0 {
		maxV = 1
	}
	barW := int(float64(value) / float64(maxV) * float64(width))
	if barW < 0 {
		barW = 0
	}
	if barW > width {
		barW = width
	}
	bar := strings.Repeat("█", barW) + strings.Repeat("░", width-barW)
	return fmt.Sprintf("%-*s %s", labelW, Trunc(label, labelW), bar)
}

// Sparkline renders a compact single-line sparkline for a slice of ints.
// The result is exactly `width` characters wide (or len(values) if shorter).
//
// Characters used (low → high): ▁▂▃▄▅▆▇█
func Sparkline(values []int, width int) string {
	const sparks = "▁▂▃▄▅▆▇█"
	sparkRunes := []rune(sparks)
	levels := len(sparkRunes) // 8

	if len(values) == 0 || width <= 0 {
		return ""
	}

	// Downsample or use as-is
	data := values
	if len(data) > width {
		// average-downsample to fit width
		data = downsample(values, width)
	}

	maxV := 1
	for _, v := range data {
		if v > maxV {
			maxV = v
		}
	}

	var sb strings.Builder
	for _, v := range data {
		idx := int(float64(v) / float64(maxV) * float64(levels-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= levels {
			idx = levels - 1
		}
		sb.WriteRune(sparkRunes[idx])
	}

	// Pad with spaces on the right if shorter than width
	result := sb.String()
	if len([]rune(result)) < width {
		result += strings.Repeat(" ", width-len([]rune(result)))
	}
	return result
}

// downsample reduces a slice to targetLen by averaging buckets.
func downsample(values []int, targetLen int) []int {
	if targetLen <= 0 {
		return nil
	}
	out := make([]int, targetLen)
	ratio := float64(len(values)) / float64(targetLen)
	for i := 0; i < targetLen; i++ {
		start := int(float64(i) * ratio)
		end := int(float64(i+1) * ratio)
		if end > len(values) {
			end = len(values)
		}
		if start >= end {
			out[i] = 0
			continue
		}
		sum := 0
		for _, v := range values[start:end] {
			sum += v
		}
		out[i] = sum / (end - start)
	}
	return out
}
