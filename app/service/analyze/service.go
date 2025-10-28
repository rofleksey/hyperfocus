package analyze

import (
	"context"
	"errors"
	"hyperfocus/app/client/frame_grabber"
	"hyperfocus/app/client/twitch_live"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/util/dbd"
	"hyperfocus/app/util/telemetry"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/rofleksey/meg"
	"github.com/samber/do"
	"github.com/samber/oops"
)

var serviceName = "analyze"

type Service struct {
	cfg           *config.Config
	queries       database.TxQueries
	tracing       *telemetry.Tracing
	liveClient    *twitch_live.Client
	frameGrabber  *frame_grabber.Client
	imageAnalyzer *dbd.ImageAnalyzer
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
	taskChan := make(chan string)

	for range s.cfg.Processing.WorkerCount {
		wg.Go(func() {
			s.runWorker(ctx, &wg, taskChan)
		})
	}

	for _, stream := range streams {
		taskChan <- stream.ID
	}

	close(taskChan)
	wg.Wait()

	slog.Debug("Processing finished",
		slog.Duration("duration", time.Since(started)),
	)

	return nil
}

func (s *Service) runWorker(ctx context.Context, wg *sync.WaitGroup, taskChan chan string) {
	defer wg.Done()

	for channelName := range taskChan {
		if err := s.processChannel(ctx, channelName); err != nil {
			slog.ErrorContext(ctx, "Error processing channel",
				slog.String("channel_name", channelName),
				slog.Any("error", err),
			)
		}

		time.Sleep(time.Second)
	}
}

func (s *Service) processChannel(ctx context.Context, channelName string) error {
	started := time.Now()
	timeout := time.Duration(s.cfg.Processing.Timeout) * time.Second

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	streamQualities, err := s.liveClient.GetM3U8(ctx, channelName)
	if err != nil {
		if errors.Is(err, twitch_live.ErrNotFound) {
			slog.Debug("Skipping offline channel",
				slog.String("channel_name", channelName),
			)

			return nil
		}
		return oops.Errorf("GetM3U8: %v", err)
	}
	if len(streamQualities) == 0 {
		return oops.Errorf("No stream qualities found")
	}

	quality, err := selectOptimalStreamQuality(streamQualities)
	if err != nil {
		return oops.Errorf("selectOptimalStreamQuality: %v", err)
	}

	url := quality.URL

	frameImg, err := s.frameGrabber.GrabFrameFromM3U8(ctx, url)
	if err != nil {
		return oops.Errorf("GrabFrameFromM3U8: %v", err)
	}

	data, err := s.imageAnalyzer.AnalyzeImage(ctx, frameImg)
	if err != nil {
		return oops.Errorf("AnalyzeBytes: %v", err)
	}

	if err = s.queries.UpdateStreamData(ctx, database.UpdateStreamDataParams{
		ID:          channelName,
		PlayerNames: meg.NonNilSlice(data.Usernames),
	}); err != nil {
		return oops.Errorf("UpdateStreamData: %v", err)
	}

	slog.Debug("Finished processing channel",
		slog.String("channel_name", channelName),
		slog.Duration("duration", time.Since(started)),
		slog.Int("usernames_count", len(data.Usernames)),
	)

	//if len(data.Usernames) != 4 {
	//	util.SaveDebugImage(frameImg, channelName)
	//}

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

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()
}
