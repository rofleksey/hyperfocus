package dbd

import (
	"image"
	"regexp"
	"sort"
	"strings"
)

var multipleSpacesRegex = regexp.MustCompile(`[a-zA-Z0-9_-]*?\s{2,}`)
var htmlTagsRegex = regexp.MustCompile(`<[^>]*>`)

type SubImager interface {
	SubImage(r image.Rectangle) image.Image
}

func keepLongestFour(strings []string) []string {
	if len(strings) <= 4 {
		return strings
	}

	type stringWithIndex struct {
		str   string
		index int
	}

	indexedStrings := make([]stringWithIndex, len(strings))
	for i, s := range strings {
		indexedStrings[i] = stringWithIndex{str: s, index: i}
	}

	sort.Slice(indexedStrings, func(i, j int) bool {
		if len(indexedStrings[i].str) != len(indexedStrings[j].str) {
			return len(indexedStrings[i].str) > len(indexedStrings[j].str)
		}
		return indexedStrings[i].index < indexedStrings[j].index
	})

	longestFour := indexedStrings[:4]

	sort.Slice(longestFour, func(i, j int) bool {
		return longestFour[i].index < longestFour[j].index
	})

	result := make([]string, 4)
	for i, item := range longestFour {
		result[i] = item.str
	}

	return result
}

func purifyUsername(s string) string {
	s = multipleSpacesRegex.ReplaceAllString(s, " ")
	s = htmlTagsRegex.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}
