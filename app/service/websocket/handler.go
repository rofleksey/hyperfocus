package websocket

import (
	"bytes"
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/service/pubsub"
	"log/slog"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/jellydator/ttlcache/v3"
)

var pingMsg = []byte("ping")

type ConnectionHandler struct {
	conn      *websocket.Conn
	channels  []string
	pubSub    *pubsub.Service
	writeChan chan api.IdMessage
	ctx       context.Context
	cancel    context.CancelFunc
}

func (h *ConnectionHandler) Handle() {
	defer h.cleanup()

	h.setupSubscriptions()
	h.startWriter()
	h.runReader()
}

func (h *ConnectionHandler) writeMessage(msg api.IdMessage) bool {
	select {
	case <-h.ctx.Done():
		return false
	case h.writeChan <- msg:
	default:
		slog.Warn("Write channel full, dropping message")
	}

	return true
}

func (h *ConnectionHandler) setupSubscriptions() {
	for _, channel := range h.channels {
		sub := h.pubSub.Subscribe(channel, func(data any) {
			defer func() {
				if err := recover(); err != nil {
					slog.Error("Panic in subscription handler", slog.Any("error", err))
				}
			}()

			idMsg, ok := data.(api.IdMessage)
			if !ok {
				slog.Error("Failed to cast pubsub message to IdMessage",
					slog.Any("data", data),
				)
				return
			}

			if !h.writeMessage(idMsg) {
				return
			}
		})
		defer h.pubSub.Unsubscribe(sub)
	}
}

func (h *ConnectionHandler) startWriter() {
	idCache := ttlcache.New[string, struct{}]()
	go idCache.Start()
	defer idCache.Stop()

	go func() {
		for {
			select {
			case <-h.ctx.Done():
				return
			case data := <-h.writeChan:
				id := data.GetId()

				if id != "" && idCache.Has(id) {
					continue
				}

				idCache.Set(id, struct{}{}, time.Minute)

				_ = h.conn.SetWriteDeadline(time.Now().Add(1 * time.Minute))
				_ = h.conn.WriteJSON(data)
			}
		}
	}()
}

func (h *ConnectionHandler) runReader() {
	for {
		_ = h.conn.SetReadDeadline(time.Now().Add(1 * time.Minute))

		_, msg, err := h.conn.ReadMessage()
		if err != nil {
			return
		}

		if bytes.Equal(msg, pingMsg) {
			if !h.writeMessage(&api.WsMessage{
				Event: "pong",
			}) {
				return
			}
		}
	}
}

func (h *ConnectionHandler) cleanup() {
	h.cancel()
	close(h.writeChan)
}
