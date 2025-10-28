package paddle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hyperfocus/app/config"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/samber/do"
)

type Client struct {
	cfg    *config.Config
	client *http.Client
}

type OCRResponse struct {
	Results []struct {
		Text       string  `json:"text"`
		Confidence float64 `json:"confidence"`
	} `json:"results"`
	Error string `json:"error"`
}

func NewClient(di *do.Injector) (*Client, error) {
	return &Client{
		cfg: do.MustInvoke[*config.Config](di),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.Paddle.BaseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Recognize(ctx context.Context, img image.Image) (*OCRResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if err = png.Encode(part, img); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Paddle.BaseURL+"/ocr", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OCR request failed with status: %d", resp.StatusCode)
	}

	var ocrResp OCRResponse
	err = json.NewDecoder(resp.Body).Decode(&ocrResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode OCR response: %w", err)
	}

	if ocrResp.Error != "" {
		return nil, fmt.Errorf("OCR error: %s", ocrResp.Error)
	}

	return &ocrResp, nil
}
