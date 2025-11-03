package analyze

import (
	"context"
	"errors"
	"hyperfocus/app/client/twitch_live"
	"hyperfocus/app/database"
	"image"
	"strconv"
	"strings"
	"sync"

	"github.com/samber/oops"
)

type StreamTask struct {
	Index  int
	Stream database.Stream

	Mutex sync.Mutex
	Frame image.Image
	Error bool
}

func (s *Service) obtainStreamFrame(ctx context.Context, stream database.Stream, proxy string) (image.Image, error) {
	// try to use cached stream url first
	if stream.Url != nil {
		frameImg, err := s.frameGrabber.GrabFrameFromM3U8(ctx, *stream.Url, proxy)
		if err == nil {
			return frameImg, nil
		}
	}

	//if err := s.liveLimiter.Wait(ctx); err != nil {
	//	return nil, fmt.Errorf("liveLimiter.Wait: %w", err)
	//}

	streamQualities, err := s.liveClient.GetM3U8(ctx, stream.ID, proxy)
	if err != nil {
		if errors.Is(err, twitch_live.ErrNotFound) {
			return nil, nil
		}

		return nil, oops.Errorf("GetM3U8: %v", err)
	}
	if len(streamQualities) == 0 {
		return nil, oops.Errorf("No stream qualities found")
	}

	quality, err := selectOptimalStreamQuality(streamQualities)
	if err != nil {
		return nil, oops.Errorf("selectOptimalStreamQuality: %v", err)
	}

	url := quality.URL

	frameImg, err := s.frameGrabber.GrabFrameFromM3U8(ctx, url, proxy)
	if err != nil {
		return nil, oops.Errorf("GrabFrameFromM3U8: %v", err)
	}

	// cache stream url
	if err = s.queries.UpdateStreamUrl(ctx, database.UpdateStreamUrlParams{
		ID:  stream.ID,
		Url: &url,
	}); err != nil {
		return nil, oops.Errorf("UpdateStreamUrl: %v", err)
	}

	return frameImg, err
}

func selectOptimalStreamQuality(arr []twitch_live.StreamQuality) (twitch_live.StreamQuality, error) {
	var result twitch_live.StreamQuality
	var maxResolution int

	for _, q := range arr {
		if q.Resolution == "1920x1080" {
			return q, nil
		}
	}

	for _, q := range arr {
		if q.Resolution == "1280x720" {
			return q, nil
		}
	}

	for _, q := range arr {
		split := strings.Split(q.Resolution, "x")
		if len(split) != 2 {
			continue
		}

		width, _ := strconv.Atoi(split[0])
		if width <= maxResolution {
			continue
		}

		result = q
		maxResolution = width
	}

	if maxResolution == 0 {
		return twitch_live.StreamQuality{}, errors.New("could not find stream quality")
	}

	return result, nil
}
