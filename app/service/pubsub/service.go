package pubsub

import (
	"hyperfocus/app/api"

	"github.com/samber/do"
	"github.com/simonfxr/pubsub"
)

type Service struct {
	bus *pubsub.Bus
}

func New(_ *do.Injector) (*Service, error) {
	return &Service{
		bus: pubsub.NewBus(),
	}, nil
}

func (s *Service) Subscribe(channel string, callback func(message any)) *pubsub.Subscription {
	return s.bus.Subscribe(channel, callback)
}

func (s *Service) Unsubscribe(sub *pubsub.Subscription) {
	s.bus.Unsubscribe(sub)
}

func (s *Service) Publish(channel string, message api.IdMessage) {
	s.bus.Publish(channel, message)
}
