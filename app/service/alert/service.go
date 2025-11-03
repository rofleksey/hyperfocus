package alert

import (
	"context"
	"fmt"
	"hyperfocus/app/client/twitch"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/service/search"
	"hyperfocus/app/util/telemetry"
	"log/slog"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/jellydator/ttlcache/v3"
	"github.com/rofleksey/meg"
	"github.com/samber/do"
)

var serviceName = "alert"

var notificationFormat = "@%s you might be playing vs a streamer '%s', please check"

type Service struct {
	cfg           *config.Config
	queries       database.TxQueries
	tracing       *telemetry.Tracing
	searchService *search.Service
	client        *twitch.Client

	alertCache *ttlcache.Cache[TriggerKey, struct{}]
}

type TriggerKey struct {
	AlertSteamer  string `json:"alert_steamer"`
	TargetSteamer string `json:"target_steamer"`
}

func New(di *do.Injector) (*Service, error) {
	alertCache := ttlcache.New[TriggerKey, struct{}]()
	go alertCache.Start()

	return &Service{
		cfg:           do.MustInvoke[*config.Config](di),
		queries:       do.MustInvoke[database.TxQueries](di),
		tracing:       do.MustInvoke[*telemetry.Tracing](di),
		searchService: do.MustInvoke[*search.Service](di),
		client:        do.MustInvoke[*twitch.Client](di),
		alertCache:    alertCache,
	}, nil
}

func (s *Service) doCheck(ctx context.Context) {
	for _, entry := range s.cfg.Alert.List {
		if err := s.checkEntry(ctx, entry); err != nil {
			slog.ErrorContext(ctx, "Failed to check alert entry",
				slog.String("streamer", entry.Streamer),
				slog.Any("error", err),
			)
		}
	}
}

func (s *Service) checkEntry(ctx context.Context, entry config.AlertEntry) error {
	targetStream, err := s.processQueries(ctx, entry.Streamer, entry.Queries)
	if err != nil {
		return fmt.Errorf("processQueries: %w", err)
	}
	if targetStream == "" {
		return nil
	}

	key := TriggerKey{
		AlertSteamer:  entry.Streamer,
		TargetSteamer: targetStream,
	}

	ttlDuration := time.Duration(s.cfg.Alert.TTL) * time.Second

	_, exists := s.alertCache.GetOrSet(key, struct{}{}, ttlcache.WithTTL[TriggerKey, struct{}](ttlDuration))
	if exists {
		return nil
	}

	notificationText := fmt.Sprintf(notificationFormat, entry.Streamer, targetStream)

	if s.cfg.Alert.DryRun {
		slog.Info("Would alert about streamsniping, but dry-run mode is enabled",
			slog.String("message", notificationText),
			slog.Bool("telegram", true),
		)

		return nil
	}

	if err = s.client.SendMessage(entry.Streamer, notificationText); err != nil {
		return fmt.Errorf("SendMessage: %w", err)
	}

	slog.Info("Streamsniping alert",
		slog.String("message", notificationText),
		slog.Bool("telegram", true),
	)

	return nil
}

func (s *Service) processQueries(ctx context.Context, alertStreamer string, queries []string) (string, error) {
	for _, query := range queries {
		searchResults, err := s.searchService.Search(ctx, query)
		if err != nil {
			return "", fmt.Errorf("searchService.Search(%s): %w", query, err)
		}

		// ignore the streamer themselves
		searchResults = pie.Filter(searchResults, func(stream database.Stream) bool {
			return stream.ID != alertStreamer
		})

		if len(searchResults) > 0 {
			return searchResults[0].ID, nil
		}
	}

	return "", nil
}

func (s *Service) RunFetchLoop(ctx context.Context) {
	interval := time.Duration(s.cfg.Alert.CheckInterval) * time.Second
	meg.RunTicker(ctx, interval, func() {
		s.doCheck(ctx)
	})
}
