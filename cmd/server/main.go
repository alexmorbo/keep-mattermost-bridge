package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/alexmorbo/keep-mattermost-bridge/application/usecase"
	"github.com/alexmorbo/keep-mattermost-bridge/infrastructure/config"
	"github.com/alexmorbo/keep-mattermost-bridge/infrastructure/keep"
	"github.com/alexmorbo/keep-mattermost-bridge/infrastructure/mattermost"
	"github.com/alexmorbo/keep-mattermost-bridge/infrastructure/messagebuilder"
	"github.com/alexmorbo/keep-mattermost-bridge/infrastructure/valkey"
	httpInterface "github.com/alexmorbo/keep-mattermost-bridge/interface/http"
	"github.com/alexmorbo/keep-mattermost-bridge/interface/http/handler"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

func main() {
	log := logger.New("info")
	slog.SetDefault(log)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log = logger.New(cfg.Server.LogLevel)
	slog.SetDefault(log)

	log.Info("starting keep-mattermost-bridge", "addr", cfg.Server.Addr())

	fileCfg, err := config.LoadFromFile(cfg.ConfigPath)
	if err != nil {
		log.Error("failed to load file config", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		cancel()
		log.Error("failed to connect to valkey", "error", err)
		os.Exit(1)
	}
	cancel()
	log.Info("connected to valkey", "addr", cfg.Redis.Addr)

	postRepo := valkey.NewPostRepository(redisClient, log.With("component", "valkey"))

	mmClient := mattermost.NewClient(cfg.Mattermost.URL, cfg.Mattermost.Token, log.With("component", "mattermost_client"))

	keepClient := keep.NewClient(cfg.Keep.URL, cfg.Keep.APIKey, log.With("component", "keep_client"))

	msgBuilder := messagebuilder.NewBuilder(fileCfg)

	handleAlertUC := usecase.NewHandleAlertUseCase(
		postRepo,
		mmClient,
		msgBuilder,
		fileCfg,
		cfg.Keep.UIURL,
		cfg.CallbackURL,
		log.With("component", "handle_alert_usecase"),
	)

	handleCallbackUC := usecase.NewHandleCallbackUseCase(
		postRepo,
		keepClient,
		mmClient,
		msgBuilder,
		cfg.Keep.UIURL,
		cfg.CallbackURL,
		log.With("component", "handle_callback_usecase"),
	)

	webhookHandler := handler.NewWebhookHandler(handleAlertUC, log.With("component", "webhook_handler"))
	callbackHandler := handler.NewCallbackHandler(handleCallbackUC)
	healthHandler := handler.NewHealthHandler(postRepo)

	gin.SetMode(gin.ReleaseMode)
	router := httpInterface.NewRouter(log, webhookHandler, callbackHandler, healthHandler)

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	log.Info("server started", "addr", cfg.Server.Addr())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Error("server error", "error", err)
	case <-quit:
		log.Info("shutting down...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", "error", err)
	}

	handleCallbackUC.Wait()

	if err := redisClient.Close(); err != nil {
		log.Error("failed to close redis client", "error", err)
	}

	log.Info("server stopped")
}
