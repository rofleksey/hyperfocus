package frame_grabber

import (
	"fmt"
	"strconv"
	"strings"
)

const defaultAdDuration = 15.0

func (c *Client) analyzeAds(m3u8Content string) (float64, error) {
	lines := strings.Split(m3u8Content, "\n")

	// Track multiple ad indicators
	var hasAds bool
	var adDuration float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for stitched ad with duration (most reliable indicator)
		if strings.Contains(line, `CLASS="twitch-stitched-ad"`) {
			hasAds = true
			if strings.Contains(line, `DURATION=`) {
				dur, err := c.extractDuration(line)
				if err == nil && dur > adDuration {
					adDuration = dur
				}
			}
		}

		// Check for ad quartile markers
		if strings.Contains(line, `CLASS="twitch-ad-quartile"`) {
			hasAds = true
		}

		// Check for preroll ads
		if strings.Contains(line, `X-TV-TWITCH-AD-ROLL-TYPE="PREROLL"`) {
			hasAds = true
			// Default preroll duration if not specified
			if adDuration == 0 {
				adDuration = defaultAdDuration
			}
		}

		// Check for non-live stream sources (ad content)
		if strings.Contains(line, `X-TV-TWITCH-STREAM-SOURCE=`) {
			if !strings.Contains(line, `X-TV-TWITCH-STREAM-SOURCE="live"`) {
				hasAds = true
				if adDuration == 0 {
					adDuration = defaultAdDuration
				}
			}
		}
	}

	if hasAds {
		// Use detected duration or fallback to typical ad duration
		if adDuration > 0 {
			return adDuration, nil
		}
		return defaultAdDuration, nil // Typical ad duration fallback
	}

	return 0, nil // No ads detected
}

func (c *Client) extractDuration(line string) (float64, error) {
	durStart := strings.Index(line, `DURATION=`)
	if durStart == -1 {
		return 0, fmt.Errorf("DURATION not found")
	}

	durPart := line[durStart+9:]

	// Find the end of duration value (comma or end of line)
	endPos := len(durPart)
	for i, char := range durPart {
		if char == ',' || char == '"' {
			endPos = i
			break
		}
	}

	if endPos > 0 {
		durPart = durPart[:endPos]
	}

	// Remove any remaining quotes
	durPart = strings.Trim(durPart, `"`)

	duration, err := strconv.ParseFloat(durPart, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration '%s': %v", durPart, err)
	}

	return duration, nil
}
