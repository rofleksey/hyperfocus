package limits

import (
	"context"
	"hyperfocus/app/util"
	"log/slog"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/samber/do"
	"golang.org/x/time/rate"
)

type Service struct {
	limitersMap *ttlcache.Cache[string, *rate.Limiter]
	alertCache  *ttlcache.Cache[string, struct{}]
}

func New(_ *do.Injector) (*Service, error) {
	limitersMap := ttlcache.New[string, *rate.Limiter]()
	alertCache := ttlcache.New[string, struct{}]()

	go limitersMap.Start()
	go alertCache.Start()

	return &Service{
		limitersMap: limitersMap,
		alertCache:  alertCache,
	}, nil
}

func (s *Service) getLimiter(key string, count int, duration time.Duration) *rate.Limiter {
	cacheItem := s.limitersMap.Get(key)
	if cacheItem != nil {
		return cacheItem.Value()
	}

	rps := float64(count) / duration.Seconds()
	limiter := rate.NewLimiter(rate.Limit(rps), count)

	s.limitersMap.Set(key, limiter, duration)

	return limiter
}

func (s *Service) wait(ctx context.Context, key string, count int, duration time.Duration) error {
	limiter := s.getLimiter(key, count, duration)
	if limiter == nil {
		return nil
	}

	return limiter.Wait(ctx) //nolint:wrapcheck
}

func (s *Service) allow(ctx context.Context, key string, count int, duration time.Duration) bool {
	limiter := s.getLimiter(key, count, duration)
	if limiter == nil {
		return true
	}

	allow := limiter.Allow()
	if !allow {
		_, exists := s.alertCache.GetOrSet(key, struct{}{}, ttlcache.WithTTL[string, struct{}](10*time.Minute))

		if !exists {
			slog.ErrorContext(ctx, "Reached allow limit on key",
				slog.String("key", key),
				slog.Int("count", count),
			)
		}
	}

	return allow
}

func (s *Service) WaitIpRps(ctx context.Context, key string, count int) error {
	ip, _ := ctx.Value(util.IpContextKey).(string)

	return s.wait(ctx, key+"_"+ip+"_rps", count, time.Second)
}

func (s *Service) WaitIpRpm(ctx context.Context, key string, count int) error {
	ip, _ := ctx.Value(util.IpContextKey).(string)

	return s.wait(ctx, key+"_"+ip+"_rpm", count, time.Minute)
}

func (s *Service) WaitGlobalRps(ctx context.Context, key string, count int) error {
	return s.wait(ctx, key+"_rps", count, time.Second)
}

func (s *Service) WaitGlobalRpm(ctx context.Context, key string, count int) error {
	return s.wait(ctx, key+"_rpm", count, time.Minute)
}

func (s *Service) AllowIpRps(ctx context.Context, key string, count int) bool {
	ip, _ := ctx.Value(util.IpContextKey).(string)

	return s.allow(ctx, key+"_"+ip+"_rps", count, time.Second)
}

func (s *Service) AllowIpRpm(ctx context.Context, key string, count int) bool {
	ip, _ := ctx.Value(util.IpContextKey).(string)

	return s.allow(ctx, key+"_"+ip+"_rpm", count, time.Minute)
}

func (s *Service) AllowGlobalRps(ctx context.Context, key string, count int) bool {
	return s.allow(ctx, key+"_rps", count, time.Second)
}

func (s *Service) AllowGlobalRpm(ctx context.Context, key string, count int) bool {
	return s.allow(ctx, key+"_rpm", count, time.Minute)
}
