package frame_grabber

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"time"

	"github.com/samber/do"
)

type Client struct{}

func NewClient(di *do.Injector) (*Client, error) {
	return &Client{}, nil
}

func (c *Client) GrabFrameFromM3U8(ctx context.Context, url string) (image.Image, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", url,
		"-vf", "scale=1920:1080",
		"-vframes", "1",
		"-f", "image2pipe",
		"-c", "png",
		"-",
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("ffmpeg timeout after 30 seconds")
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("ffmpeg execution failed: %v, output: %s", err, stderr.String())
		}
	}

	output := stdout.Bytes()
	if len(output) == 0 {
		return nil, fmt.Errorf("no frame data captured from ffmpeg")
	}

	result, err := png.Decode(bytes.NewReader(output))
	if err != nil {
		return nil, fmt.Errorf("invalid PNG data from ffmpeg: %v", err)
	}

	size := result.Bounds().Size()
	if size.X == 0 && size.Y == 0 {
		return nil, fmt.Errorf("invalid image size")
	}

	return result, nil
}
