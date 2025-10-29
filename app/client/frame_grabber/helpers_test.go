package frame_grabber

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeAds(t *testing.T) {
	tests := []struct {
		filename     string
		wantDuration float64
	}{
		{
			filename:     "test_dataset/ads.m3u8",
			wantDuration: 15.235,
		},
		{
			filename:     "test_dataset/ads_1.m3u8",
			wantDuration: 15.235,
		},
		{
			filename:     "test_dataset/no_ads.m3u8",
			wantDuration: 0,
		},
	}

	client := &Client{}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			content, err := os.ReadFile(tt.filename)
			require.NoError(t, err)

			gotDuration, err := client.analyzeAds(string(content))
			require.NoError(t, err)

			assert.Equal(t, tt.wantDuration, gotDuration)
		})
	}
}
