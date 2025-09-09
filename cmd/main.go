package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"tezjet/config"
	"tezjet/internal/handler"
	"tezjet/internal/repository"
	"tezjet/traits/database"
	"tezjet/traits/logger"

	"github.com/go-telegram/bot"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	zapLogger, err := logger.NewLogger()
	if err != nil {
		panic(err)
	}
	defer zapLogger.Sync()

	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		zapLogger.Error("error init config", zap.Error(err))
		return
	}

	// Validate configuration
	if err := cfg.ValidateConfig(); err != nil {
		zapLogger.Error("invalid configuration", zap.Error(err))
		return
	}

	zapLogger.Info("Starting TezJet application",
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
		zap.String("db_name", cfg.DBName),
	)

	// Initialize database
	db, err := database.InitDatabase(cfg, zapLogger)
	if err != nil {
		zapLogger.Error("failed to initialize database", zap.Error(err))
		return
	}
	defer db.Close()

	// Create database tables
	if err := database.CreateTables(db, zapLogger); err != nil {
		zapLogger.Error("failed to create tables", zap.Error(err))
		return
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db, zapLogger)
	driverRepo := repository.NewDriverRepository(db, zapLogger)

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create handler with repositories
	handl := handler.NewHandler(cfg, zapLogger, db, userRepo, driverRepo)

	// Create bot instance
	opts := []bot.Option{
		bot.WithDefaultHandler(handl.DefaultHandler),
	}

	b, err := bot.New(cfg.Token, opts...)
	if err != nil {
		zapLogger.Error("error creating bot", zap.Error(err))
		return
	}

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-stop
		zapLogger.Info("Shutdown signal received")
		cancel()
	}()

	// Start web server
	go handl.StartWebServer(ctx, b)
	zapLogger.Info("Web server started", zap.String("address", cfg.GetServerAddress()))

	// Start bot
	zapLogger.Info("Bot started successfully")
	b.Start(ctx)

	zapLogger.Info("Application stopped successfully")
}
