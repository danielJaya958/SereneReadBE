package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"booksReader/internal/config"
	"booksReader/internal/handlers"
	customMiddleware "booksReader/internal/middleware"
	"booksReader/internal/platform/database"
	"booksReader/internal/repository"
	"booksReader/internal/service"
	"booksReader/internal/worker" // TAMBAHAN: Import package worker Anda

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq" // TAMBAHAN: Import library asynq

	_ "booksReader/docs" // ← TAMBAHKAN INI

	httpSwagger "github.com/swaggo/http-swagger" // ← TAMBAHKAN INI
)

// @title           BooksReader API
// @version         1.0
// @description     API untuk aplikasi membaca buku PDF
// @host            localhost:8081
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
func main() {
	// =========================================================================
	// 1. Inisialisasi Konfigurasi & Database
	// =========================================================================
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Gagal load config: %v", err)
	}

	// 1.a. Init PostgreSQL
	db, err := database.NewConnection(cfg.DBUrl)
	if err != nil {
		log.Fatalf("Gagal connect ke database: %v", err)
	}
	log.Println("✅ Database PostgreSQL Terkoneksi!")

	// 1.b. Init Redis (Untuk Caching)
	rdb, err := database.NewRedisConnection(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("Gagal connect ke Redis (%s): %v. Pastikan Redis jalan: wsl -d Ubuntu -- redis-server", cfg.RedisAddr, err)
	}
	log.Println("✅ Database Redis (Cache) Terkoneksi!")

	// =========================================================================
	// 1.c. TAMBAHAN: Inisialisasi Asynq (Client & Server)
	// =========================================================================
	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisAddr}

	// ASYNQ CLIENT (Pencatat Tugas / Producer)
	asynqClient := asynq.NewClient(redisOpt)
	log.Println("✅ Asynq Client Terkoneksi!")

	// ASYNQ SERVER (Pekerja Belakang Layar / Consumer)
	asynqServer := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 5, // Bisa mengerjakan 5 tugas thumbnail secara bersamaan
		},
	)

	// =========================================================================
	// 2. Dependency Injection
	// =========================================================================
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handlers.NewUserHandler(userService)

	bookRepo := repository.NewBookRepository(db)
	// PERBAIKAN: Masukkan 'rdb' DAN 'asynqClient' ke dalam parameter NewBookService
	bookService := service.NewBookService(bookRepo, rdb, asynqClient)
	bookHandler := handlers.NewBookHandler(bookService)

	bookmarkRepo := repository.NewBookmarkRepository(db)
	bookmarkService := service.NewBookmarkService(bookmarkRepo, bookRepo)
	bookmarkHandler := handlers.NewBookmarkHandler(bookmarkService)

	statsRepo := repository.NewStatsRepository(db)
	statsService := service.NewStatsService(statsRepo, bookRepo)
	statsHandler := handlers.NewStatsHandler(statsService)

	// =========================================================================
	// 3. Daftarkan Tugas Worker (Router Khusus Background Job)
	// =========================================================================
	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TypeGenerateThumbnail, func(ctx context.Context, t *asynq.Task) error {
		return worker.HandleGenerateThumbnailTask(ctx, t, bookRepo, rdb)
	})

	// Jalankan Worker di dalam Goroutine (Berjalan Paralel/Background)
	go func() {
		log.Println("👷 Asynq Worker berjalan dan siap menerima tugas...")
		if err := asynqServer.Run(mux); err != nil {
			log.Fatalf("Worker gagal berjalan: %v", err)
		}
	}()

	// Buat ulang sampul yang belum jadi (misalnya server mati sebelum worker selesai)
	go worker.EnqueueMissingThumbnails(context.Background(), bookRepo, asynqClient)

	// =========================================================================
	// 4. Setup Router Chi & Middleware (API)
	// =========================================================================
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(customMiddleware.CancelPrevious())
	r.Use(middleware.Compress(5, "application/json"))

	// =========================================================================
	// 5. Routing
	// =========================================================================
	// Swagger UI
	r.Get("/swagger/*", httpSwagger.WrapHandler)
	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Post("/register", userHandler.Register)
			r.Post("/login", userHandler.Login)
		})

		r.Group(func(r chi.Router) {
			r.Use(customMiddleware.AuthMiddleware())

			r.Post("/books", bookHandler.Create)
			r.Get("/books", bookHandler.GetAll)
			r.Get("/books/{id}", bookHandler.GetDetail)
			r.Get("/books/{id}/content", bookHandler.GetContent)
			r.Get("/books/{id}/file", bookHandler.GetFile)
			r.Put("/books/{id}", bookHandler.Update)
			r.Patch("/books/{id}", bookHandler.Patch)
			r.Delete("/books/{id}", bookHandler.Delete)
			r.Put("/books/{id}/favorite", bookHandler.SetFavorite)
			r.Patch("/books/{id}/progress", bookHandler.UpdateProgress)

			r.Get("/books/{id}/bookmarks", bookmarkHandler.List)
			r.Post("/books/{id}/bookmarks", bookmarkHandler.Create)
			r.Delete("/books/{id}/bookmarks/{bookmarkId}", bookmarkHandler.Delete)

			r.Post("/reading-sessions", statsHandler.SaveSessions)
			r.Get("/stats/summary", statsHandler.Summary)
			r.Get("/stats/daily", statsHandler.Daily)
			r.Get("/stats/books", statsHandler.Books)
		})
	})

	// PDF tidak lagi di-serve statis: akses lewat GET /api/v1/books/{id}/file (dicek pemiliknya)
	fileServerThumb := http.FileServer(http.Dir("./uploads/thumbnails/"))
	r.Handle("/uploads/thumbnails/*", http.StripPrefix("/uploads/thumbnails/", fileServerThumb))

	// =========================================================================
	// 6. Konfigurasi Server HTTP (Graceful Shutdown)
	// =========================================================================
	port := cfg.ServerPort
	if port == "" {
		port = "8081" // Port Anda diset ke 8081
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Printf("🚀 Server HTTP booksReader berjalan di http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("\n🛑 Sinyal Shutdown diterima. Mematikan server secara perlahan...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Matikan HTTP Server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server dipaksa mati karena error: %v", err)
	}

	// 2. TAMBAHAN: Matikan Asynq Server (Pekerja) dengan sopan
	asynqServer.Stop()
	log.Println("✅ Asynq Worker dihentikan.")

	// 3. TAMBAHAN: Tutup Asynq Client
	asynqClient.Close()
	log.Println("✅ Asynq Client ditutup.")

	// 4. Tutup Redis
	rdb.Close()
	log.Println("✅ Koneksi Redis ditutup.")

	// 5. Tutup Database
	db.Close()
	log.Println("✅ Koneksi PostgreSQL ditutup.")

	log.Println("👋 Server berhasil dimatikan dengan aman. Sampai jumpa!")
}
