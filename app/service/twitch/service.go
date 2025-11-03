package twitch

import (
	"context"
	"fmt"
	"hyperfocus/app/client/twitch"
	"hyperfocus/app/database"
	"hyperfocus/app/util/telemetry"
	"log/slog"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/nicklaw5/helix/v2"
	"github.com/samber/do"
	"github.com/samber/oops"
)

var serviceName = "twitch"

type Service struct {
	queries database.TxQueries
	tracing *telemetry.Tracing
	client  *twitch.Client
}

func New(di *do.Injector) (*Service, error) {
	return &Service{
		queries: do.MustInvoke[database.TxQueries](di),
		tracing: do.MustInvoke[*telemetry.Tracing](di),
		client:  do.MustInvoke[*twitch.Client](di),
	}, nil
}

func (s *Service) doFetch(ctx context.Context) error {
	slog.Debug("Starting fetch")

	started := time.Now()

	var after string
	streamMap := make(map[string]struct{})

	for {
		chunk, err := s.fetchChunkWithRetry(ctx, after)
		if err != nil {
			return oops.Errorf("fetchChunkWithRetry: %w", err)
		}
		if chunk.Pagination.Cursor == "" {
			break
		}

		//slog.Debug("Got a chunk of streams",
		//	slog.Int("count", len(chunk.Streams)),
		//)

		for _, stream := range chunk.Streams {
			streamID := strings.ToLower(stream.UserLogin)
			streamMap[streamID] = struct{}{}

			if err = s.queries.CreateStream(ctx, database.CreateStreamParams{
				ID:      streamID,
				Updated: started,
			}); err != nil {
				return oops.Errorf("CreateStream: %w", err)
			}

			if err = s.queries.SetStreamOnline(ctx, database.SetStreamOnlineParams{
				ID:      streamID,
				Updated: started,
			}); err != nil {
				return oops.Errorf("UpdateStreamOnline: %w", err)
			}
		}

		after = chunk.Pagination.Cursor

		select {
		case <-ctx.Done():
			return oops.Errorf("fetchAllLiveStreams: context canceled")
		case <-time.After(3 * time.Second):
		}
	}

	if err := s.queries.UpdateStaleStreams(ctx, started); err != nil {
		return oops.Errorf("UpdateStaleStreams: %w", err)
	}

	slog.Debug("Fetch finished",
		slog.Duration("duration", time.Since(started)),
		slog.Int("count", len(streamMap)),
	)

	return nil
}

func (s *Service) fetchChunkWithRetry(ctx context.Context, after string) (*helix.ManyStreams, error) {
	var result *helix.ManyStreams

	attempts := 3

	err := retry.Do(func() error {
		chunk, err := s.client.GetLiveDBDStreams(after)
		if err != nil {
			return oops.Errorf("GetLiveDBDStreams: %w", err)
		}

		result = &chunk

		return nil
	}, retry.Context(ctx), retry.Attempts(uint(attempts)), retry.Delay(time.Second*5))
	if err != nil {
		return nil, fmt.Errorf("retry.Do: %w", err)
	}

	return result, nil
}

func (s *Service) RunFetchLoop(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := s.doFetch(ctx); err != nil {
				slog.ErrorContext(ctx, "Failed to fetch streams",
					slog.Any("error", err),
				)
			}

			// TODO: reduce time in production
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()
}
