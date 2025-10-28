package util

import (
	"image"
	"image/png"
	"log/slog"
	"os"
)

func SaveDebugImageLocal(img image.Image, name string) {
	file, err := os.Create(name + ".png")
	if err != nil {
		slog.Error("Failed to create file for saving debug image",
			slog.String("name", name),
			slog.Any("error", err),
		)
		return
	}
	defer file.Close()

	if err = png.Encode(file, img); err != nil {
		slog.Error("Failed to save debug image",
			slog.String("name", name),
			slog.Any("error", err),
		)
		return
	}
}

func SaveDebugImage(img image.Image, name string) {
	file, err := os.Create("debug_dataset/" + name + ".png")
	if err != nil {
		slog.Error("Failed to create file for saving debug image",
			slog.String("name", name),
			slog.Any("error", err),
		)
		return
	}
	defer file.Close()

	if err = png.Encode(file, img); err != nil {
		slog.Error("Failed to save debug image",
			slog.String("name", name),
			slog.Any("error", err),
		)
		return
	}
}
