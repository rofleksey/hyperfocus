package middleware

import (
	"context"
	"hyperfocus/app/config"
	"hyperfocus/app/service/auth"
	"hyperfocus/app/util"
	"hyperfocus/app/util/telemetry"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/elliotchance/pie/v2"
	sentryotel "github.com/getsentry/sentry-go/otel"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rofleksey/meg"
	"github.com/samber/do"
	slogfiber "github.com/samber/slog-fiber"
)

func FiberMiddleware(app *fiber.App, di *do.Injector) {
	cfg := do.MustInvoke[*config.Config](di)
	tel := do.MustInvoke[*telemetry.Telemetry](di)
	authService := do.MustInvoke[*auth.Service](di)

	staticOrigins := []string{
		cfg.BaseURL,
		"capacitor://localhost", "http://localhost", "https://localhost", "http://localhost:4321", "http://localhost:5173",
		"http://localhost:1234", "http://localhost:3000", "http://localhost:9000", "http://localhost:8080",
	}

	// cors
	app.Use(cors.New(cors.Config{
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Api-Key, Sentry-Trace, Baggage",
		AllowMethods:     "POST, GET, OPTIONS, DELETE, PUT, PATCH, HEAD",
		AllowCredentials: true,
		AllowOriginsFunc: func(origin string) bool {
			return pie.Contains(staticOrigins, origin)
		},
	}))

	app.Use(otelfiber.Middleware(
		otelfiber.WithMeterProvider(tel.MeterProvider),
		otelfiber.WithTracerProvider(tel.TracerProvider),
		otelfiber.WithPropagators(sentryotel.NewSentryPropagator()),
		otelfiber.WithCollectClientIP(true),
	))

	// inject fiber ctx into user ctx
	app.Use(func(ctx *fiber.Ctx) error {
		ctx.SetUserContext(util.InjectFiberIntoContext(ctx.UserContext(), ctx))

		return ctx.Next()
	})

	// retrieve user ip
	app.Use(func(ctx *fiber.Ctx) error {
		ctx.SetUserContext(context.WithValue(ctx.UserContext(), util.IpContextKey, ctx.IP()))

		return ctx.Next()
	})

	ignorePaths := []string{"/api/healthz"}

	// log requests
	app.Use(slogfiber.NewWithConfig(slog.Default(), slogfiber.Config{
		Filters: []slogfiber.Filter{
			func(c *fiber.Ctx) bool {
				return !slices.Contains(ignorePaths, c.Path())
			},
			func(ctx *fiber.Ctx) bool {
				reqMethod := strings.ToLower(string(ctx.Context().Method()))
				return !(reqMethod == "get" && (ctx.Response().StatusCode() == http.StatusOK || ctx.Response().StatusCode() == http.StatusNotModified || ctx.Response().StatusCode() == http.StatusPartialContent)) //nolint:staticcheck
			},
		},
		WithTraceID: true,
	}))

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(ctx *fiber.Ctx, e any) {
			stackStr := meg.TrimSuffixToNRunes(string(debug.Stack()), 2048)

			slog.ErrorContext(ctx.Context(), "Panic",
				slog.Any("error", e),
				slog.String("stack", stackStr),
			)
		},
	}))

	// extract authorization from JWT
	app.Use(jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(cfg.Auth.JWT.Secret)},
		SuccessHandler: func(ctx *fiber.Ctx) error {
			tokenOpt := ctx.Locals("user")
			if tokenOpt == nil {
				return ctx.Next()
			}

			token := tokenOpt.(*jwt.Token) //nolint:forcetypeassert

			tokenUser, err := authService.ValidateToken(ctx.UserContext(), token)
			if err != nil {
				return ctx.Next()
			}

			ctx.Locals(string(util.UserContextKey), tokenUser)

			newUserCtx := context.WithValue(ctx.UserContext(), util.UserContextKey, tokenUser)
			newUserCtx = context.WithValue(newUserCtx, util.UsernameContextKey, tokenUser.Username)
			ctx.SetUserContext(newUserCtx)

			return ctx.Next()
		},
		ErrorHandler: func(ctx *fiber.Ctx, _ error) error {
			return ctx.Next()
		},
		TokenLookup: "query:token,cookie:hyperfocus_auth",
		AuthScheme:  "Bearer",
	}))

	// verify api key
	app.Use(func(ctx *fiber.Ctx) error {
		apiKey := ctx.Get("X-Api-Key")
		if apiKey == "" {
			return ctx.Next()
		}

		apiUser, err := authService.ValidateApiKey(ctx.UserContext(), apiKey)
		if err != nil {
			return ctx.Next()
		}

		ctx.Locals(string(util.UserContextKey), apiUser)

		newUserCtx := context.WithValue(ctx.UserContext(), util.UserContextKey, apiUser)
		newUserCtx = context.WithValue(newUserCtx, util.UsernameContextKey, apiUser.Username)
		ctx.SetUserContext(newUserCtx)

		return ctx.Next()
	})
}
