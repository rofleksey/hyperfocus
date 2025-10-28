package dbd

import (
	"context"
	"fmt"
	"hyperfocus/app/client/magick"
	"hyperfocus/app/client/paddle"
	"hyperfocus/app/config"
	"hyperfocus/app/util"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageAnalyzer_AnalyzeImage(t *testing.T) {
	tests := []struct {
		imagePath         string
		expectedUsernames []string
	}{
		{
			imagePath:         "test_dataset/k0per1s_1.jpg",
			expectedUsernames: []string{"kOper1s live :-)", "PkNoLuck", "livia", "ANGELDEAD pro"},
		},
		{
			imagePath:         "test_dataset/xweza_1.png",
			expectedUsernames: []string{"Vise47s", "tris-divergente", "Claudette Morel_01", "Leon S. Kennedy_02"},
		},
		{
			imagePath:         "test_dataset/bigwill82_1.png",
			expectedUsernames: []string{"Bigwill82", "Flamingo0-_-", "Spooky Scary Fishl...", "Clappnz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.imagePath, func(t *testing.T) {
			di := do.New()

			cfg, err := config.Load("../../../config.yaml")
			require.NoError(t, err)

			do.ProvideValue(di, cfg)
			do.Provide(di, paddle.NewClient)
			do.Provide(di, magick.NewClient)
			do.Provide(di, NewImageAnalyzer)

			file, err := os.Open(tt.imagePath)
			require.NoError(t, err)
			defer file.Close()

			var img image.Image
			if strings.HasSuffix(tt.imagePath, ".jpg") {
				img, err = jpeg.Decode(file)
			} else {
				img, err = png.Decode(file)
			}
			require.NoError(t, err)

			analyzer := do.MustInvoke[*ImageAnalyzer](di)

			data, err := analyzer.AnalyzeImage(context.Background(), img)
			require.NoError(t, err)
			require.NotNil(t, data)

			require.Len(t, data.Usernames, 4)

			for i, expectedUsername := range tt.expectedUsernames {
				if util.LevenshtainDistance(data.Usernames[i], expectedUsername) > 2 {
					assert.Fail(t, fmt.Sprintf("Usernames mismatch: required %s, found %s", expectedUsername, data.Usernames[i]))
				}
			}
		})
	}
}
