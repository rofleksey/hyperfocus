package dbd

import (
	"bytes"
	"context"
	"fmt"
	"hyperfocus/app/client/magick"
	"hyperfocus/app/client/paddle"
	"hyperfocus/app/util"
	"image"
	"image/png"
	"testing"

	"github.com/samber/do"
)

type ImageAnalyzer struct {
	paddleClient *paddle.Client
	magickClient *magick.Client
}

func NewImageAnalyzer(di *do.Injector) (*ImageAnalyzer, error) {
	return &ImageAnalyzer{
		paddleClient: do.MustInvoke[*paddle.Client](di),
		magickClient: do.MustInvoke[*magick.Client](di),
	}, nil
}

type AnalyzeResult struct {
	Usernames []string
}

func (a *ImageAnalyzer) AnalyzeImage(ctx context.Context, img image.Image) (*AnalyzeResult, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	usernames, err := a.analyzeUsernames(ctx, img)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze usernames: %w", err)
	}

	return &AnalyzeResult{
		Usernames: usernames,
	}, nil
}

func (a *ImageAnalyzer) analyzeUsernames(ctx context.Context, img image.Image) ([]string, error) {
	hudImage, err := a.magickClient.CropImage(ctx, img, 145, 420, 378-145, 835-420)
	if err != nil {
		return nil, fmt.Errorf("CropImage: %w", err)
	}

	hudImage, err = a.magickClient.ProcessImageForOCR(ctx, hudImage)
	if err != nil {
		return nil, fmt.Errorf("ProcessImageForOCR: %w", err)
	}

	if testing.Testing() {
		util.SaveDebugImageLocal(hudImage, "hudImage")
	}

	res, err := a.paddleClient.Recognize(ctx, hudImage)
	if err != nil {
		return nil, fmt.Errorf("failed to recognize image: %w", err)
	}

	return a.parseUsernames(res), nil
}

func (a *ImageAnalyzer) parseUsernames(ocrResult *paddle.OCRResponse) []string {
	var usernames []string

	for _, res := range ocrResult.Results {
		if res.Confidence < 0.5 {
			continue
		}

		username := purifyUsername(res.Text)
		if a.isValidUsername(username) {
			usernames = append(usernames, username)
		}
	}

	return keepLongestFour(usernames)
}

func (a *ImageAnalyzer) isValidUsername(username string) bool {
	if len(username) < 3 {
		return false
	}

	return true
}
