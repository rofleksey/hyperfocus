package analyze

import (
	"errors"
	"hyperfocus/app/client/twitch_live"
	"strconv"
	"strings"
)

func selectOptimalStreamQuality(arr []twitch_live.StreamQuality) (twitch_live.StreamQuality, error) {
	var result twitch_live.StreamQuality
	var maxResolution int

	for _, q := range arr {
		if q.Resolution == "1920x1080" {
			return q, nil
		}
	}

	for _, q := range arr {
		split := strings.Split(q.Resolution, "x")
		if len(split) != 2 {
			continue
		}

		width, _ := strconv.Atoi(split[0])
		if width <= maxResolution {
			continue
		}

		result = q
		maxResolution = width
	}

	if maxResolution == 0 {
		return twitch_live.StreamQuality{}, errors.New("could not find stream quality")
	}

	return result, nil
}
