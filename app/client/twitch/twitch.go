package twitch

import (
	"context"
	"fmt"
	"hyperfocus/app/config"
	"log/slog"
	"net/http"
	"time"

	"github.com/nicklaw5/helix/v2"
	"github.com/samber/do"
)

const dbdID = "491487"

type Client struct {
	cfg        *config.Config
	userClient *helix.Client
}

func NewClient(di *do.Injector) (*Client, error) {
	ctx := do.MustInvoke[context.Context](di)
	cfg := do.MustInvoke[*config.Config](di)

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	helixClient, err := helix.NewClientWithContext(ctx, &helix.Options{
		ClientID:     cfg.Twitch.ClientID,
		ClientSecret: cfg.Twitch.ClientSecret,
		RefreshToken: cfg.Twitch.RefreshToken,
		HTTPClient:   httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create helix client: %w", err)
	}

	resp, err := helixClient.RefreshUserAccessToken(cfg.Twitch.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get token: status %d: %s", resp.StatusCode, resp.ErrorMessage)
	}

	accessToken := resp.Data.AccessToken
	helixClient.SetUserAccessToken(accessToken)

	return &Client{
		cfg:        cfg,
		userClient: helixClient,
	}, nil
}

func (c *Client) GetLiveDBDStreams(after ...string) (helix.ManyStreams, error) {
	var afterParam string
	if len(after) > 0 {
		afterParam = after[0]
	}

	resp, err := c.userClient.GetStreams(&helix.StreamsParams{
		After:   afterParam,
		First:   100,
		GameIDs: []string{dbdID},
		Type:    "live",
	})
	if err != nil {
		return helix.ManyStreams{}, fmt.Errorf("GetStreams: %w", err)
	}
	if resp.StatusCode != 200 {
		return helix.ManyStreams{}, fmt.Errorf("GetStreams: status %d: %s", resp.StatusCode, resp.ErrorMessage)
	}

	return resp.Data, nil
}

func (c *Client) GetUserIDByUsername(username string) (string, error) {
	resp, err := c.userClient.GetUsers(&helix.UsersParams{
		Logins: []string{username},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get user info: status %d: %s", resp.StatusCode, resp.ErrorMessage)
	}

	if len(resp.Data.Users) == 0 {
		return "", fmt.Errorf("failed to get user info: no users found")
	}

	return resp.Data.Users[0].ID, nil
}

func (c *Client) SendMessage(channel, text string) error {
	broadcasterID, err := c.GetUserIDByUsername(channel)
	if err != nil {
		return fmt.Errorf("failed to get broadcaster id: %w", err)
	}

	senderID, err := c.GetUserIDByUsername(c.cfg.Twitch.Username)
	if err != nil {
		return fmt.Errorf("failed to get sender id: %w", err)
	}

	resp, err := c.userClient.SendChatMessage(&helix.SendChatMessageParams{
		BroadcasterID: broadcasterID,
		SenderID:      senderID,
		Message:       text,
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to send message: status %d: %s", resp.StatusCode, resp.ErrorMessage)
	}

	return nil
}

func (c *Client) GetStreamStartedAt(username string) (time.Time, error) {
	userID, err := c.GetUserIDByUsername(username)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get user id: %w", err)
	}

	resp, err := c.userClient.GetStreams(&helix.StreamsParams{
		UserIDs: []string{userID},
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get stream info: %w", err)
	}
	if resp.StatusCode != 200 {
		return time.Time{}, fmt.Errorf("failed to get stream info: status %d: %s", resp.StatusCode, resp.ErrorMessage)
	}

	if len(resp.Data.Streams) == 0 {
		return time.Time{}, fmt.Errorf("stream is not live")
	}

	stream := resp.Data.Streams[0]

	return stream.StartedAt, nil
}

func (c *Client) refreshToken() {
	slog.Debug("Refreshing twitch access token",
		slog.String("username", c.cfg.Twitch.Username),
	)

	resp, err := c.userClient.RefreshUserAccessToken(c.cfg.Twitch.RefreshToken)
	if err != nil {
		slog.Error("Failed to refresh user access token", slog.Any("error", err))
		return
	}
	if resp.StatusCode != 200 {
		slog.Error("Failed to refresh access token", slog.Int("status", resp.StatusCode), slog.String("error", resp.ErrorMessage))
		return
	}

	c.userClient.SetUserAccessToken(resp.Data.AccessToken)

	slog.Debug("Twitch access token refreshed successfully",
		slog.String("username", c.cfg.Twitch.Username),
	)
}

func (c *Client) RunRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refreshToken()
		}
	}
}
