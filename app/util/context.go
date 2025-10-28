package util

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

type ContextKey string

func (c ContextKey) String() string {
	return "hyperfocus_" + string(c)
}

var FiberContextKey = ContextKey("fiber")
var UserContextKey = ContextKey("user")
var UsernameContextKey ContextKey = "username"
var IpContextKey ContextKey = "ip"

func InjectFiberIntoContext(ctx context.Context, c *fiber.Ctx) context.Context {
	return context.WithValue(ctx, FiberContextKey, c)
}

func GetFiberFromContext(ctx context.Context) *fiber.Ctx {
	return ctx.Value(FiberContextKey).(*fiber.Ctx) //nolint:forcetypeassert
}
