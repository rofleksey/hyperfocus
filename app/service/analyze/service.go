package analyze

import (
	"context"
	"fmt"
	"hyperfocus/app/client/frame_grabber"
	"hyperfocus/app/client/twitch_live"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/util"
	"hyperfocus/app/util/dbd"
	"hyperfocus/app/util/telemetry"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/rofleksey/meg"
	"github.com/samber/do"
	"github.com/samber/oops"
	"golang.org/x/time/rate"
)

var serviceName = "analyze"

type Service struct {
	cfg           *config.Config
	queries       database.TxQueries
	tracing       *telemetry.Tracing
	liveClient    *twitch_live.Client
	frameGrabber  *frame_grabber.Client
	imageAnalyzer *dbd.ImageAnalyzer

	liveLimiter *rate.Limiter
}

func New(di *do.Injector) (*Service, error) {
	os.Mkdir("debug_dataset", 0750)

	return &Service{
		cfg:           do.MustInvoke[*config.Config](di),
		queries:       do.MustInvoke[database.TxQueries](di),
		tracing:       do.MustInvoke[*telemetry.Tracing](di),
		liveClient:    do.MustInvoke[*twitch_live.Client](di),
		frameGrabber:  do.MustInvoke[*frame_grabber.Client](di),
		imageAnalyzer: do.MustInvoke[*dbd.ImageAnalyzer](di),

		liveLimiter: rate.NewLimiter(rate.Limit(1), 1), // 1rps
	}, nil
}

func (s *Service) doProcessing(ctx context.Context) error {
	slog.Debug("Starting processing",
		slog.Int("worker_count", s.cfg.Processing.WorkerCount),
	)

	started := time.Now()

	streams, err := s.queries.GetOnlineStreams(ctx)
	if err != nil {
		return oops.Errorf("GetOnlineStreams: %v", err)
	}

	var wg sync.WaitGroup
	taskChan := make(chan database.Stream)

	for range s.cfg.Processing.WorkerCount {
		wg.Go(func() {
			s.runWorker(ctx, taskChan)
		})
	}

	for _, stream := range streams {
		taskChan <- stream
	}

	close(taskChan)
	wg.Wait()

	slog.Debug("Processing finished",
		slog.Duration("duration", time.Since(started)),
	)

	return nil
}

func (s *Service) runWorker(ctx context.Context, taskChan chan database.Stream) {
	for stream := range taskChan {
		if err := s.processChannel(ctx, stream); err != nil {
			slog.ErrorContext(ctx, "Error processing channel",
				slog.String("channel_name", stream.ID),
				slog.Any("error", err),
			)
		}
	}
}

func (s *Service) processChannel(ctx context.Context, stream database.Stream) error {
	started := time.Now()
	timeout := time.Duration(s.cfg.Processing.Timeout) * time.Second

	frameImg, err := s.obtainStreamFrame(ctx, stream)
	if err != nil {
		return oops.Errorf("obtainStreamFrame: %v", err)
	}

	if frameImg == nil {
		slog.Debug("Skipping offline channel",
			slog.String("channel_name", stream.ID),
		)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := s.imageAnalyzer.AnalyzeImage(ctx, frameImg)
	if err != nil {
		return oops.Errorf("AnalyzeBytes: %v", err)
	}

	if err = s.queries.UpdateStreamData(ctx, database.UpdateStreamDataParams{
		ID:          stream.ID,
		PlayerNames: meg.NonNilSlice(data.Usernames),
	}); err != nil {
		return oops.Errorf("UpdateStreamData: %v", err)
	}

	slog.Debug("Finished processing channel",
		slog.String("channel_name", stream.ID),
		slog.Duration("duration", time.Since(started)),
		slog.Int("usernames_count", len(data.Usernames)),
	)

	if meg.Environment != "production" && len(data.Usernames) != 4 {
		util.SaveDebugImage(frameImg, fmt.Sprintf("%s-%d", stream.ID, len(data.Usernames)))
	}

	return nil
}

func (s *Service) RunProcessLoop(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := s.doProcessing(ctx); err != nil {
				slog.ErrorContext(ctx, "Processing failed",
					slog.Any("error", err),
				)
			}

			// TODO: remove in production
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()
}
