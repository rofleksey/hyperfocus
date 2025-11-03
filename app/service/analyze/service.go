package analyze

import (
	"context"
	"hyperfocus/app/client/frame_grabber"
	"hyperfocus/app/client/twitch_live"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/util/dbd"
	"hyperfocus/app/util/telemetry"
	"image"
	"log/slog"
	"math/rand"
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
	started := time.Now()

	streams, err := s.queries.GetOnlineStreams(ctx)
	if err != nil {
		return oops.Errorf("GetOnlineStreams: %v", err)
	}
	if len(streams) == 0 {
		return nil
	}

	slog.Debug("Starting processing",
		slog.Int("fetch_worker_count", s.cfg.Processing.FetchWorkerCount),
		slog.Int("process_worker_count", s.cfg.Processing.ProcessWorkerCount),
		slog.Int("proxy_count", len(s.cfg.Proxy.List)),
		slog.Int("task_count", len(streams)),
	)

	var wg sync.WaitGroup

	fetchChan := make(chan *StreamTask, s.cfg.Processing.FetchWorkerCount)
	processChanInternal := make(chan *StreamTask, s.cfg.Processing.FrameBufferSize)
	processChan := make(chan *StreamTask, s.cfg.Processing.ProcessWorkerCount)

	for range s.cfg.Processing.FetchWorkerCount {
		wg.Go(func() {
			s.runFetchWorker(ctx, fetchChan, processChanInternal)
		})
	}

	for range s.cfg.Processing.ProcessWorkerCount {
		wg.Go(func() {
			s.runProcessWorker(ctx, processChan)
		})
	}

	wg.Go(func() {
		for index, stream := range streams {
			fetchChan <- &StreamTask{
				Index:  index,
				Stream: stream,
			}
		}
		close(fetchChan)
	})

	wg.Go(func() {
		counter := 0

		for task := range processChanInternal {
			processChan <- task
			counter++

			if counter == len(streams) {
				break
			}
		}

		close(processChanInternal)
		close(processChan)
	})

	wg.Wait()

	slog.Debug("Processing finished",
		slog.Duration("duration", time.Since(started)),
		slog.Int("count", len(streams)),
		slog.Float64("rate", float64(len(streams))/time.Since(started).Seconds()),
	)

	return nil
}

func (s *Service) runFetchWorker(ctx context.Context, taskChan chan *StreamTask, resultChan chan *StreamTask) {
	for task := range taskChan {
		frameImg, err := s.fetchChannelFrame(ctx, task)
		if err != nil {
			slog.ErrorContext(ctx, "Error fetching channel frame",
				slog.String("channel_name", task.Stream.ID),
				slog.Any("error", err),
			)
		}

		task.Mutex.Lock()
		task.Frame = frameImg
		task.Error = err != nil
		task.Mutex.Unlock()

		resultChan <- task
	}
}

func (s *Service) runProcessWorker(ctx context.Context, taskChan chan *StreamTask) {
	for task := range taskChan {
		if err := s.processChannel(ctx, task); err != nil {
			slog.ErrorContext(ctx, "Error processing channel",
				slog.String("channel_name", task.Stream.ID),
				slog.Any("error", err),
			)
		}
	}
}

func (s *Service) fetchChannelFrame(ctx context.Context, task *StreamTask) (image.Image, error) {
	//started := time.Now()
	timeout := time.Duration(s.cfg.Processing.FetchTimeout) * time.Second

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var proxy string
	if len(s.cfg.Proxy.List) > 0 {
		proxy = s.cfg.Proxy.List[rand.Intn(len(s.cfg.Proxy.List))]
	}

	frameImg, err := s.obtainStreamFrame(ctx, task.Stream, proxy)
	if err != nil {
		return nil, oops.Errorf("obtainStreamFrame: %v", err)
	}

	// skipping offline channel
	if frameImg == nil {
		return nil, nil
	}

	//slog.Debug("Finished fetching channel frame",
	//	slog.Int("index", task.Index),
	//	slog.String("channel_name", task.Stream.ID),
	//	slog.Duration("duration", time.Since(started)),
	//)

	return frameImg, nil
}

func (s *Service) processChannel(ctx context.Context, task *StreamTask) error {
	task.Mutex.Lock()
	frameImg := task.Frame
	taskErr := task.Error
	task.Mutex.Unlock()

	if frameImg == nil || taskErr {
		return nil
	}

	//started := time.Now()
	timeout := time.Duration(s.cfg.Processing.ProcessTimeout) * time.Second

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := s.imageAnalyzer.AnalyzeImage(ctx, frameImg)
	if err != nil {
		return oops.Errorf("AnalyzeBytes: %v", err)
	}

	if err = s.queries.UpdateStreamData(ctx, database.UpdateStreamDataParams{
		ID:          task.Stream.ID,
		PlayerNames: meg.NonNilSlice(data.Usernames),
	}); err != nil {
		return oops.Errorf("UpdateStreamData: %v", err)
	}

	//slog.Debug("Finished processing channel",
	//	slog.Int("index", task.Index),
	//	slog.String("channel_name", task.Stream.ID),
	//	slog.Duration("duration", time.Since(started)),
	//	slog.Int("usernames_count", len(data.Usernames)),
	//)

	//if meg.Environment != "production" && len(data.Usernames) != 4 {
	//	util.SaveDebugImage(frameImg, fmt.Sprintf("%s-%d", task.Stream.ID, len(data.Usernames)))
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
		}
	}()
}
