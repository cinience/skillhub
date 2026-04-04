package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/auth"
	"github.com/cinience/skillhub/internal/cli"
	"github.com/cinience/skillhub/internal/config"
	"github.com/cinience/skillhub/internal/model"
	"github.com/cinience/skillhub/internal/repository"
	"github.com/cinience/skillhub/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		runServer()
		return
	}

	switch os.Args[1] {
	case "serve":
		runServer()
	case "admin":
		handleAdmin()
	case "login":
		cli.Login(os.Args[2:])
	case "whoami":
		cli.WhoAmI(os.Args[2:])
	case "search":
		cli.Search(os.Args[2:])
	case "list":
		cli.List(os.Args[2:])
	case "inspect":
		cli.Inspect(os.Args[2:])
	case "install":
		cli.Install(os.Args[2:])
	case "uninstall":
		cli.Uninstall(os.Args[2:])
	case "installed":
		cli.Installed(os.Args[2:])
	case "update":
		cli.Update(os.Args[2:])
	case "publish":
		cli.Publish(os.Args[2:])
	case "help", "--help", "-h":
		cli.PrintUsage()
	default:
		// Backward compatible: unknown subcommand starts the server
		runServer()
	}
}

func runServer() {
	configPath := "configs/skillhub.yaml"
	if v := os.Getenv("SKILLHUB_CONFIG"); v != "" {
		configPath = v
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("SkillHub starting on %s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := srv.Run(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("server stopped")
}

func handleAdmin() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  skillhub admin create-user --handle <handle> --role <role> [--password <password>]")
		fmt.Println("  skillhub admin create-token --user <handle> --label <label>")
		fmt.Println("  skillhub admin set-password --user <handle> --password <password>")
		os.Exit(1)
	}

	configPath := "configs/skillhub.yaml"
	if v := os.Getenv("SKILLHUB_CONFIG"); v != "" {
		configPath = v
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := repository.NewDB(cfg.Database)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	ctx := context.Background()

	switch os.Args[2] {
	case "create-user":
		handle := getFlag(os.Args[3:], "--handle")
		role := getFlag(os.Args[3:], "--role")
		password := getFlag(os.Args[3:], "--password")
		if handle == "" {
			log.Fatal("--handle is required")
		}
		if role == "" {
			role = "admin"
		}

		userRepo := repository.NewUserRepo(db)
		user := &model.User{
			ID:     uuid.New(),
			Handle: handle,
			Role:   role,
		}
		if err := userRepo.Create(ctx, user); err != nil {
			log.Fatalf("failed to create user: %v", err)
		}

		if password != "" {
			tokenRepo := repository.NewTokenRepo(db)
			authSvc := auth.NewService(tokenRepo, userRepo)
			if err := authSvc.SetPassword(ctx, user.ID, password); err != nil {
				log.Fatalf("failed to set password: %v", err)
			}
			fmt.Printf("User created: %s (ID: %s, role: %s, password set)\n", handle, user.ID, role)
		} else {
			fmt.Printf("User created: %s (ID: %s, role: %s)\n", handle, user.ID, role)
		}

	case "create-token":
		userHandle := getFlag(os.Args[3:], "--user")
		label := getFlag(os.Args[3:], "--label")
		if userHandle == "" {
			log.Fatal("--user is required")
		}
		if label == "" {
			label = "CLI"
		}

		userRepo := repository.NewUserRepo(db)
		tokenRepo := repository.NewTokenRepo(db)
		authSvc := auth.NewService(tokenRepo, userRepo)

		user, err := userRepo.GetByHandle(ctx, userHandle)
		if err != nil || user == nil {
			log.Fatalf("user not found: %s", userHandle)
		}

		rawToken, _, err := authSvc.CreateToken(ctx, user.ID, label)
		if err != nil {
			log.Fatalf("failed to create token: %v", err)
		}
		fmt.Printf("Token created for %s:\n%s\n", userHandle, rawToken)

	case "set-password":
		userHandle := getFlag(os.Args[3:], "--user")
		password := getFlag(os.Args[3:], "--password")
		if userHandle == "" {
			log.Fatal("--user is required")
		}
		if password == "" {
			log.Fatal("--password is required")
		}

		userRepo := repository.NewUserRepo(db)
		tokenRepo := repository.NewTokenRepo(db)
		authSvc := auth.NewService(tokenRepo, userRepo)

		user, err := userRepo.GetByHandle(ctx, userHandle)
		if err != nil || user == nil {
			log.Fatalf("user not found: %s", userHandle)
		}

		if err := authSvc.SetPassword(ctx, user.ID, password); err != nil {
			log.Fatalf("failed to set password: %v", err)
		}
		fmt.Printf("Password set for %s\n", userHandle)

	default:
		fmt.Printf("Unknown admin command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func getFlag(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
