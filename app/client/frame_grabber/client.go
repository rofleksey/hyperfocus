package frame_grabber

import (
	"bytes"
	"context"
	"fmt"
	"hyperfocus/app/config"
	"image"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/image/bmp"

	"github.com/samber/do"
)

const clientId = "kimne78kx3ncx6brgo4mv6wki5h1ko"

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewClient(di *do.Injector) (*Client, error) {
	return &Client{
		cfg: do.MustInvoke[*config.Config](di),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (c *Client) GrabFrameFromM3U8(ctx context.Context, url string) (image.Image, error) {
	adDuration, err := c.getAdDuration(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("getAdDuration: %v", err)
	}

	skipTime := "0"
	if adDuration > 0 {
		skipTime = fmt.Sprintf("%.1f", adDuration+1)
		slog.Debug("Detected ads, skipping...",
			slog.String("duration", skipTime),
			slog.String("url", url),
		)
	}

	headers := strings.Join([]string{
		"User-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		"Accept: */*",
		"Accept-Language: en-US,en;q=0.5",
		"Origin: https://www.twitch.tv",
		"Referer: https://www.twitch.tv/",
		"Client-Id:" + clientId,
		//"Device-Id" + c.cfg.Twitch.BrowserDeviceID,
		"Authorization: OAuth " + c.cfg.Twitch.BrowserOauthToken,
	}, "\r\n")

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-headers", headers,
		"-i", url,
		"-ss", skipTime,
		"-vf", "scale=1920:1080",
		"-vframes", "1",
		"-f", "image2pipe",
		"-c", "bmp",
		"-",
		"-skip_frame", "nokey", // Skip non-keyframes for faster seeking
		"-threads", "1", // Use single thread to avoid overhead
		"-noaccurate_seek",    // Faster but less precise seeking
		"-flags", "low_delay", // Reduce buffering delays
		"-avioflags", "direct", // Reduce buffering
		"-fflags", "nobuffer+flush_packets", // Minimal buffering
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
	case <-time.After(time.Minute):
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

	result, err := bmp.Decode(bytes.NewReader(output))
	if err != nil {
		return nil, fmt.Errorf("invalid PNG data from ffmpeg: %v", err)
	}

	size := result.Bounds().Size()
	if size.X == 0 && size.Y == 0 {
		return nil, fmt.Errorf("invalid image size")
	}

	return result, nil
}

func (c *Client) getAdDuration(ctx context.Context, m3u8URL string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", m3u8URL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	// Set appropriate headers for m3u8 request
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Origin", "https://www.twitch.tv")
	req.Header.Set("Referer", "https://www.twitch.tv/")
	req.Header.Set("Client-Id", clientId)
	//req.Header.Set("Device-Id", c.cfg.Twitch.BrowserDeviceID)
	req.Header.Set("Authorization", "OAuth "+c.cfg.Twitch.BrowserOauthToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch m3u8: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("m3u8 fetch failed with status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Read with limit to prevent excessive memory usage
	content, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return 0, fmt.Errorf("failed to read m3u8 content: %v", err)
	}

	return c.analyzeAds(string(content))
}
