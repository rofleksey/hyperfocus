package cmd

import (
	"context"
	"fmt"
	"hyperfocus/app/api"
	"hyperfocus/app/api/controller"
	"hyperfocus/app/api/middleware"
	"hyperfocus/app/api/routes"
	"hyperfocus/app/client/frame_grabber"
	"hyperfocus/app/client/magick"
	"hyperfocus/app/client/paddle"
	twitchC "hyperfocus/app/client/twitch"
	"hyperfocus/app/client/twitch_live"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/database/migration"
	"hyperfocus/app/service/alert"
	"hyperfocus/app/service/analyze"
	"hyperfocus/app/service/limits"
	"hyperfocus/app/service/search"
	"hyperfocus/app/service/twitch"
	"hyperfocus/app/util/dbd"
	"hyperfocus/app/util/mylog"
	"hyperfocus/app/util/telemetry"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do"
	"github.com/spf13/cobra"
)

var configPath string

var Server = &cobra.Command{
	Use:   "server",
	Short: "Run server",
	Run:   runServer,
}

func init() {
	Server.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "Path to config yaml file (required)")
}

func runServer(_ *cobra.Command, _ []string) {
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	di := do.New()
	do.ProvideValue(di, appCtx)

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("Failed to load config",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:gocritic
		return
	}
	do.ProvideValue(di, cfg)

	if err = telemetry.InitSentry(cfg); err != nil {
		slog.Error("Failed to init sentry",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	defer sentry.Flush(3 * time.Second)

	tel, err := telemetry.Init(cfg)
	if err != nil {
		slog.Error("Failed to init telemetry",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	defer tel.Shutdown(appCtx)
	do.ProvideValue(di, tel)

	if err = mylog.Init(cfg, tel); err != nil {
		slog.Error("Failed to init logging",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	slog.InfoContext(appCtx, "Starting service...",
		slog.Bool("telegram", true),
	)

	metrics, err := telemetry.NewMetrics(cfg, tel.Meter)
	if err != nil {
		slog.Error("Failed to init metrics",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	do.ProvideValue(di, metrics)

	tracing := telemetry.NewTracing(cfg, tel.Tracer)
	do.ProvideValue(di, tracing)

	dbConnStr := "postgres://" + cfg.DB.User + ":" + cfg.DB.Pass + "@" + cfg.DB.Host + "/" + cfg.DB.Database + "?sslmode=disable&pool_max_conns=30&pool_min_conns=5&pool_max_conn_lifetime=1h&pool_max_conn_idle_time=30m&pool_health_check_period=1m&connect_timeout=10"

	dbConf, err := pgxpool.ParseConfig(dbConnStr)
	if err != nil {
		slog.Error("Failed to parse pgxpool config",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}

	dbConf.ConnConfig.RuntimeParams = map[string]string{
		"statement_timeout":                   "30000",
		"idle_in_transaction_session_timeout": "60000",
	}
	dbConf.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithMeterProvider(tel.MeterProvider),
		otelpgx.WithTracerProvider(tel.TracerProvider),
	)

	dbConn, err := pgxpool.NewWithConfig(appCtx, dbConf)
	if err != nil {
		slog.Error("Failed to connect to database",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	defer dbConn.Close()

	if err = otelpgx.RecordStats(dbConn); err != nil {
		slog.Error("Failed to start recording DB stats",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}

	do.ProvideValue(di, database.TxPool(dbConn))

	queries := database.New(dbConn)
	do.ProvideValue(di, database.TxQueries(queries))

	transactor := database.NewTransactor(dbConn, queries, tracing)
	do.ProvideValue(di, database.TxTransactor(transactor))

	if err = migration.Migrate(appCtx, di); err != nil {
		slog.Error("Migrations failed",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}

	do.Provide(di, twitchC.NewClient)
	do.Provide(di, twitch_live.NewClient)
	do.Provide(di, paddle.NewClient)
	do.Provide(di, frame_grabber.NewClient)
	do.Provide(di, magick.NewClient)
	do.Provide(di, dbd.NewImageAnalyzer)

	do.Provide(di, limits.New)
	do.Provide(di, twitch.New)
	do.Provide(di, analyze.New)
	do.Provide(di, search.New)
	do.Provide(di, alert.New)

	if err = do.MustInvoke[*paddle.Client](di).HealthCheck(appCtx); err != nil {
		slog.Error("PaddleOCR client init failed",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}

	go do.MustInvoke[*twitchC.Client](di).RunRefreshLoop(appCtx)
	go do.MustInvoke[*twitch.Service](di).RunFetchLoop(appCtx)
	go do.MustInvoke[*analyze.Service](di).RunProcessLoop(appCtx)
	go do.MustInvoke[*alert.Service](di).RunFetchLoop(appCtx)

	server := controller.NewStrictServer(di)
	handler := api.NewStrictHandler(server, nil)

	app := fiber.New(fiber.Config{
		AppName:               "Hyperfocus API",
		DisableStartupMessage: true,
		ErrorHandler:          middleware.ErrorHandler,
		ProxyHeader:           "X-Forwarded-For",
		ReadTimeout:           time.Second * 60,
		WriteTimeout:          time.Second * 60,
		DisableKeepalive:      false,
	})

	middleware.FiberMiddleware(app, di)
	routes.StaticRoutes(app)

	apiGroup := app.Group("/v1")
	api.RegisterHandlersWithOptions(apiGroup, handler, api.FiberServerOptions{
		BaseURL: "",
		Middlewares: []api.MiddlewareFunc{
			middleware.NewOpenAPIValidator(di),
		},
	})

	routes.NotFoundRoute(app)

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		slog.Info("Shutting down server...")

		_ = app.Shutdown()
		cancel()
	}()

	slog.Info(fmt.Sprintf("Server started on port %d", cfg.Server.HttpPort))
	if err := app.Listen(fmt.Sprintf(":%d", cfg.Server.HttpPort)); err != nil {
		slog.Warn("Server stopped",
			slog.Any("error", err),
		)
	}

	slog.Info("Waiting for services to finish...")
	_ = di.Shutdown()
}
