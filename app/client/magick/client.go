package magick

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os/exec"

	"github.com/samber/do"
	"golang.org/x/image/bmp"
)

type Client struct{}

func NewClient(di *do.Injector) (*Client, error) {
	return &Client{}, nil
}

func (c *Client) CropAndProcessForUsernames(ctx context.Context, img image.Image) (image.Image, error) {
	var inputBuf bytes.Buffer
	if err := bmp.Encode(&inputBuf, img); err != nil {
		return nil, fmt.Errorf("png.Encode: %w", err)
	}

	cmd := exec.CommandContext(ctx, "magick",
		"bmp:-",
		"-crop", fmt.Sprintf("%dx%d+%d+%d", 378-145, 835-420, 145, 420),
		"-colorspace", "Gray",
		"-auto-level",
		"(", "+clone", "-lat", "8x8+5%", ")",
		"(", "+clone", "-threshold", "60%", ")",
		"-compose", "darken", "-composite",
		"-negate",
		"-alpha", "off",
		"bmp:-",
	)

	return c.executeMagickCommand(cmd, &inputBuf)
}

func (c *Client) executeMagickCommand(cmd *exec.Cmd, inputBuf *bytes.Buffer) (image.Image, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	go func() {
		defer stdin.Close()
		stdin.Write(inputBuf.Bytes())
	}()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("magick execution failed: %w, stderr: %s", err, stderr.String())
	}

	outputImg, err := bmp.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to decode output image: %w", err)
	}

	return outputImg, nil
}
