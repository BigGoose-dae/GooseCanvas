package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goosecanvas/goosecanvas/internal/config"
	"github.com/goosecanvas/goosecanvas/internal/database"
	"github.com/goosecanvas/goosecanvas/internal/events"
	"github.com/goosecanvas/goosecanvas/internal/generation"
	"github.com/goosecanvas/goosecanvas/internal/httpapi"
	"github.com/goosecanvas/goosecanvas/internal/runtime"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	manager, err := runtime.New(db, cfg)
	if err != nil {
		log.Fatal(err)
	}
	hub := events.New()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	engine := generation.NewManaged(db, manager, hub)
	engine.Start(ctx)
	r := gin.Default()
	r.MaxMultipartMemory = 16 << 20
	api := &httpapi.API{DB: db, Runtime: manager, Events: hub}
	api.Register(r)
	webDist := filepath.Join("web", "dist")
	if info, statErr := os.Stat(webDist); statErr == nil && info.IsDir() {
		r.Static("/assets", filepath.Join(webDist, "assets"))
		r.Static("/brand", filepath.Join(webDist, "brand"))
		r.NoRoute(func(c *gin.Context) { c.File(filepath.Join(webDist, "index.html")) })
	} else {
		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"name": cfg.AppName, "message": "Web 尚未构建，请在 web 目录运行 npm run dev"})
		})
	}
	server := &http.Server{Addr: cfg.Addr, Handler: r, ReadHeaderTimeout: 10 * time.Second, BaseContext: func(_ net.Listener) context.Context { return ctx }}
	go func() {
		log.Printf("%s listening on %s", cfg.AppName, cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	_ = server.Shutdown(shutdownCtx)
	engine.Wait()
}
