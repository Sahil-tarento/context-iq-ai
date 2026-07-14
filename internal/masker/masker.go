package masker

import (
	"math"
	"regexp"
	"strings"
)

// Masker scans and masks sensitive data.
type Masker struct {
	patterns map[string]*regexp.Regexp
}

// NewMasker creates a new Masker with default patterns.
func NewMasker() *Masker {
	return &Masker{
		patterns: map[string]*regexp.Regexp{
			"AWS_KEY":       regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
			"SLACK_WEBHOOK": regexp.MustCompile(`https://hooks\.slack\.com/services/[T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+`),
			"JWT_TOKEN":     regexp.MustCompile(`\beyJ[A-Za-z0-9-_=]+\.eyJ[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*\b`),
			"EMAIL":         regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`),
			"IPV4":          regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),
			"DB_URI":        regexp.MustCompile(`\b[a-zA-Z]+://[^:]+:([^@\s]+)@(?:[a-zA-Z0-9.-]+)(?::[0-9]+)?/[^\s]*\b`),
			// Matches assignment of potential api keys/secrets
			"GENERIC_SECRET": regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|password|passwd|token|credential|auth|private[_-]?key)\b\s*[:=]\s*['"]([a-zA-Z0-9-_=]{16,128})['"]`),
		},
	}
}

// Mask replaces sensitive data in the text with placeholders.
// Returns the masked text and the total number of items masked.
func (m *Masker) Mask(text string) (string, int) {
	maskedText := text
	totalMasked := 0

	// 1. Mask specific patterns
	for label, regex := range m.patterns {
		if label == "GENERIC_SECRET" {
			// For generic secret, we only want to mask the captured secret value, not the label itself
			matches := regex.FindAllStringSubmatch(maskedText, -1)
			for _, match := range matches {
				if len(match) > 1 {
					secretVal := match[1]
					// Verify entropy to avoid false positives (e.g. standard short words)
					if calculateEntropy(secretVal) > 3.8 {
						maskedText = strings.ReplaceAll(maskedText, secretVal, "[MASKED_SECRET]")
						totalMasked++
					}
				}
			}
		} else if label == "DB_URI" {
			// For DB_URI, we only mask the password group
			matches := regex.FindAllStringSubmatch(maskedText, -1)
			for _, match := range matches {
				if len(match) > 1 {
					password := match[1]
					maskedText = strings.ReplaceAll(maskedText, password, "[MASKED_PASSWORD]")
					totalMasked++
				}
			}
		} else {
			// Replace all instances of the match with label
			matches := regex.FindAllString(maskedText, -1)
			for _, match := range matches {
				placeholder := "[" + label + "]"
				if !strings.Contains(maskedText, placeholder) { // avoid double counting if duplicate
					totalMasked++
				}
				maskedText = strings.ReplaceAll(maskedText, match, placeholder)
			}
		}
	}

	// 2. Fallback: entropy-based scan for long string tokens in code (potential high entropy keys)
	words := strings.Fields(text)
	for _, word := range words {
		// Clean quotes, commas, parentheses
		cleanWord := strings.Trim(word, `'"(),;{}[]`)
		if len(cleanWord) >= 32 && len(cleanWord) <= 128 {
			if calculateEntropy(cleanWord) > 3.2 {
				// Verify if it is not already masked
				if !strings.Contains(cleanWord, "MASKED") {
					maskedText = strings.ReplaceAll(maskedText, cleanWord, "[MASKED_HIGH_ENTROPY_KEY]")
					totalMasked++
				}
			}
		}
	}

	return maskedText, totalMasked
}

// calculateEntropy computes the Shannon entropy of a string.
func calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	charCounts := make(map[rune]float64)
	for _, r := range s {
		charCounts[r]++
	}

	entropy := 0.0
	length := float64(len(s))
	for _, count := range charCounts {
		p := count / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}
