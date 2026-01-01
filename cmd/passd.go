package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/mrshanahan/quemot-dev-auth-client/pkg/auth"
	quemotfiber "github.com/mrshanahan/quemot-dev-auth-client/pkg/fiber"
	"github.com/mrshanahan/simple-password-service/internal/cache"
	"github.com/mrshanahan/simple-password-service/internal/crypto"
	"github.com/mrshanahan/simple-password-service/internal/db"
	passddb "github.com/mrshanahan/simple-password-service/internal/db"
	"github.com/mrshanahan/simple-password-service/internal/render"
	"github.com/mrshanahan/simple-password-service/internal/utils"

	"golang.org/x/oauth2"
)

var (
	DB                       *db.PassdDb
	TokenCookieName          string = "access_token"
	TokenLocalName           string = "token"
	DefaultPassdDirectory    string = path.Join(os.Getenv("HOME"), ".passd")
	DefaultPort              int    = 5555
	DefaultPassdDatabaseName string = "passd.sqlite"
	DefaultPassdKeyFileName  string = "passd.key"
	KeySize                  int    = 32
	DefaultStaticFilesDir    string = "./assets"
)

func main() {
	if len(os.Args) > 1 && utils.Any(os.Args[1:], func(x string) bool { return x == "-h" || x == "--help" || x == "-?" }) {
		printHelp()
		os.Exit(0)
	}

	var exitCode int
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch strings.ToLower(command) {
	case "generate-key":
		var path string
		if len(os.Args) > 2 {
			path = os.Args[2]
		} else {
			path = filepath.Join(DefaultPassdDirectory, DefaultPassdKeyFileName)
		}
		exitCode = GenerateKey(path)
	case "run":
	case "":
		exitCode = Run()
	default:
		fmt.Fprintf(os.Stderr, "error: invalid command: %s\n", command)
		printHelp()
		exitCode = 1

	}
	os.Exit(exitCode)
}

func ensureConfigDirectory() error {
	if err := os.MkdirAll(DefaultPassdDirectory, 0700); err != nil {
		return fmt.Errorf("failed to create passd config directory %s: %w", DefaultPassdDirectory, err)
	}
	return nil
}

func GenerateKey(path string) int {
	absPath, err := filepath.Abs(path)
	if err != nil {
		slog.Error("unexpected error occurred while resolving local path", "path", absPath, "err", err)
		return 1
	}
	if _, err := filepath.Rel(DefaultPassdDirectory, absPath); err == nil {
		// path is under DefaultPassdDirectory - we'll create this directory if necessary
		if err := ensureConfigDirectory(); err != nil {
			slog.Error("failed to create passd config directory",
				"path", DefaultPassdDirectory,
				"err", err)
			return 1
		}
	}

	dir := filepath.Dir(absPath)
	dirinfo, err := os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		slog.Error("base directory of key path does not exist", "path", absPath)
		return 1
	} else if err != nil {
		slog.Error("failed to get information about key path directory", "path", absPath, "err", err)
		return 1
	} else if !dirinfo.IsDir() {
		slog.Error("base directory of key path is file", "path", absPath)
		return 1
	}

	_, err = os.Stat(absPath)
	if err != nil && !os.IsNotExist(err) {
		slog.Error("failed to get info about given file path", "path", absPath, "err", err)
		return 1
	} else if err == nil {
		slog.Error("given key path already exists", "path", absPath)
		return 1
	}

	key, err := crypto.GeneratePassdKey()
	if err != nil {
		slog.Error("failed to generate key", "err", err)
		return 1
	}

	if err := key.Save(absPath); err != nil {
		slog.Error("failed to write generated key back", "path", absPath, "err", err)
		return 1
	}

	return 0
}

func Run() int {
	dbPath := os.Getenv("PASSD_DB_PATH")
	if dbPath == "" {
		if err := ensureConfigDirectory(); err != nil {
			slog.Error("failed to create passd config directory",
				"err", err)
			return 1
		}
		dbPath = path.Join(DefaultPassdDirectory, DefaultPassdDatabaseName)
		slog.Info("no path provided for DB; using default",
			"path", dbPath)
	} else {
		slog.Info("given DB path", "path", dbPath)
		dbPathDir := filepath.Base(dbPath)
		if err := os.MkdirAll(dbPathDir, 0777); err != nil {
			slog.Error("failed to create custom passd DB path parent",
				"path", dbPathDir,
				"err", err)
			return 1
		}
	}

	if _, err := os.Open(dbPath); err != nil && errors.Is(err, os.ErrNotExist) {
		slog.Info("DB does not exist; it will be created during initialization",
			"path", dbPath)
	}

	keyPath := os.Getenv("PASSD_KEY_PATH")
	if keyPath == "" {
		if err := os.MkdirAll(DefaultPassdDirectory, 0777); err != nil {
			slog.Error("failed to create passd directory",
				"path", DefaultPassdDirectory,
				"err", err)
			return 1
		}
		keyPath = path.Join(DefaultPassdDirectory, DefaultPassdKeyFileName)
		slog.Info("no path provided for key path; using default",
			"path", keyPath)
	} else {
		slog.Info("given key path", "path", keyPath)
		keyPathDir := filepath.Base(keyPath)
		if err := os.MkdirAll(keyPathDir, 0777); err != nil {
			slog.Error("failed to create custom passd key path parent",
				"path", keyPathDir,
				"err", err)
			return 1
		}
	}

	var key crypto.PassdKey
	keyFile, err := os.Open(keyPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		slog.Error("key path does not exist; exiting", "path", keyPath)
		return 1
	} else if err != nil {
		slog.Error("failed to open key file", "path", keyPath, "err", err)
		return 1
	} else {
		key, err = crypto.Load(keyFile)
		keyFile.Close()
		if err != nil {
			slog.Error("failed to read key file", "path", keyPath, "err", err)
			return 1
		}
	}

	db, err := passddb.Open(dbPath, key)
	if err != nil {
		slog.Error("failed to open DB", "path", dbPath, "err", err)
		return 1
	}
	DB = db
	defer DB.Close()

	portStr := os.Getenv("PASSD_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = DefaultPort
		slog.Info("no valid port provided via PASSD_PORT, using default",
			"portStr", portStr,
			"defaultPort", port)
	} else {
		slog.Info("using custom port",
			"port", port)
	}

	staticFilesDir := os.Getenv("PASSD_STATIC_FILES_DIR")
	if staticFilesDir == "" {
		staticFilesDir = DefaultStaticFilesDir
	}

	if _, err := os.Stat(staticFilesDir); err != nil {
		if os.IsNotExist(err) {
			slog.Error("could not find static files directory",
				"path", staticFilesDir)
		} else {
			slog.Error("unknown error when attempting to find static files directory",
				"path", staticFilesDir,
				"err", err)
		}

		return 1
	}

	jsCache := cache.NewFileCache(cache.FileCacheConfig{
		RootDir: staticFilesDir,
		// TODO: Make these configurable from env vars; currently, cache is effectively off
		MetadataCheckInterval: time.Minute * 0,
		ValidityInterval:      time.Minute * 0,
	})

	// renderer, err := render.NewRenderer(map[string]string{
	// 	"ApiUrl": notesApiUrl,
	// })
	// if err != nil {
	// 	panic(fmt.Sprintf("error: failed to create renderer: %s", err))
	// }

	disableAuth := false
	disableAuthOption := strings.TrimSpace(os.Getenv("PASSD_DISABLE_AUTH"))
	if disableAuthOption != "" {
		slog.Warn("disabling authentication framework - THIS SHOULD ONLY BE RUN FOR TESTING!")
		disableAuth = true
	}

	if !disableAuth {
		authProviderUrl := os.Getenv("PASSD_AUTH_PROVIDER_URL")
		if authProviderUrl == "" {
			panic("Required value for PASSD_AUTH_PROVIDER_URL but none provided")
		}
		redirectUrl := os.Getenv("PASSD_REDIRECT_URL")
		if redirectUrl == "" {
			panic("Required value for PASSD_REDIRECT_URL but none provided")
		}
		if err := auth.InitializeAuthCodeFlow(context.Background(), "passd", authProviderUrl, redirectUrl); err != nil {
			slog.Error("failed to initialize OAuth2 auth code flow config", "err", err)
			panic(fmt.Sprintf("failed to initialize OAuth2 auth code flow config: %v", err))
		}
	} else {
		slog.Warn("skipping initialization of authentication framework", "disableAuth", disableAuth)
	}

	allowedOrigins := os.Getenv("PASSD_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}
	slog.Info("setting CORS allowed origins", "origins", allowedOrigins)

	apiUrlBase := os.Getenv("PASSD_API_BASE")
	if apiUrlBase == "" {
		apiUrlBase = "./admin/api"
	}

	renderer, err := render.NewRenderer(map[string]string{
		"ApiUrl": apiUrlBase,
	})
	if err != nil {
		panic(fmt.Sprintf("error: failed to create renderer: %s", err))
	}

	app := fiber.New()
	app.Use(requestid.New(), logger.New(), recover.New())

	// /validate - main, anonymous entrypoint to check passwords by public sites
	app.Post("/validate", func(ctx *fiber.Ctx) error {
		requestPayload := new(ValidatePasswordRequest)
		if err := ctx.BodyParser(requestPayload); err != nil {
			slog.Debug("invalid request body for validating password", "err", err)
			return ctx.Status(fiber.StatusBadRequest).JSON(ErrorResponse{"could not parse request body"})
		}
		storedPasswordHash, err := DB.LoadHash(requestPayload.Id)
		if err != nil {
			slog.Error("failed to load password hash", "id", requestPayload.Id, "err", err)
			return ctx.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{"failed to retrieve password"})
		}

		providedPasswordHash, err := crypto.Hash([]byte(requestPayload.Password))

		// TODO: "Constant"-time way of doing this comparison? Or does that not matter since
		// we're comparing hashes?
		equal := slices.Equal(storedPasswordHash, providedPasswordHash)
		return ctx.JSON(ValidatePasswordResponse{equal})
	})

	// /admin - route for editing password entries
	app.Route("/admin", func(admin fiber.Router) {
		admin.Use(func(c *fiber.Ctx) error {
			path := c.OriginalURL()
			if strings.HasSuffix(path, "/admin") {
				return c.Redirect(path + "/")
			}
			return c.Next()
		})

		if !disableAuth {
			// /admin/auth - authentication for admin route
			admin.Route("/auth", func(auth fiber.Router) {
				auth.Get("/login", quemotfiber.NewLoginController(func(c *fiber.Ctx) LoginState {
					cameFromParam := c.Query("came_from")
					var cameFrom string
					if cameFromParam != "" {
						cameFromBytes, err := base64.URLEncoding.DecodeString(cameFromParam)
						if err == nil {
							cameFrom = string(cameFromBytes)
						}
					}

					return LoginState{CameFrom: cameFrom}
				}))
				auth.Get("/logout", func(c *fiber.Ctx) error {
					// TODO: Invalidate token(s)
					c.ClearCookie(TokenCookieName)
					return c.SendString("Logout successful")
				})
				auth.Get("/callback", quemotfiber.NewCallbackController(func(c *fiber.Ctx, s LoginState, t *oauth2.Token) error {
					c.Cookie(&fiber.Cookie{
						Name:  "access_token",
						Value: t.AccessToken,
					})

					if s.CameFrom != "" {
						return c.Redirect(s.CameFrom)
					}
					return c.SendString("Login successful")
				}))
			})
		} else {
			slog.Warn("skipping registration of authentication-related endpoints", "disableAuth", disableAuth)
		}

		// /admin/api - API routes for admin
		admin.Route("/api", func(api fiber.Router) {
			api.Use(cors.New(cors.Config{
				AllowOrigins: allowedOrigins,
			}))
			if !disableAuth {
				api.Use(quemotfiber.ValidateAccessTokenMiddleware(TokenLocalName, TokenCookieName))
			} else {
				slog.Warn("skipping registration of token validation middleware", "disableAuth", disableAuth)
			}
			api.Get("/", func(ctx *fiber.Ctx) error {
				ids, err := DB.ListIds()
				if err != nil {
					slog.Error("failed to load password ids", "err", err)
					return ctx.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{"failed to load password ids"})
				}
				responsePayload := utils.Map(ids, func(id string) GetPasswordEntryResponse { return GetPasswordEntryResponse{id} })
				return ctx.JSON(responsePayload)
			})
			api.Get("/:id", func(ctx *fiber.Ctx) error {
				id := ctx.Params("id", "")
				if id == "" {
					return ctx.Status(fiber.StatusBadRequest).JSON(ErrorResponse{"id must be provided"})
				}
				password, err := DB.GetPassword(id)
				if err != nil {
					slog.Error("failed to retrieve password", "id", id, "err", err)
					return ctx.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{"failed to retrieve password"})
				}
				if password == nil {
					return ctx.SendStatus(fiber.StatusNotFound)
				}
				passwordStr := string(password)
				return ctx.JSON(GetPasswordResponse{id, passwordStr})
			})
			api.Post("/:id", func(ctx *fiber.Ctx) error {
				id := ctx.Params("id", "")
				if id == "" {
					return ctx.Status(fiber.StatusBadRequest).JSON(ErrorResponse{"id must be provided"})
				}
				requestPayload := new(UpsertPasswordRequest)
				if err := ctx.BodyParser(requestPayload); err != nil || requestPayload.Password == "" {
					slog.Debug("invalid request body for upserting password", "id", id, "err", err)
					return ctx.Status(fiber.StatusBadRequest).JSON(ErrorResponse{"could not parse request body"})
				}
				if err := DB.UpsertPassword(id, requestPayload.Password); err != nil {
					slog.Error("failed to upsert password", "id", id, "err", err)
					return ctx.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{"failed to upsert password"})
				}

				return ctx.SendStatus(fiber.StatusNoContent)
			})
			api.Delete("/:id", func(ctx *fiber.Ctx) error {
				id := ctx.Params("id", "")
				if id == "" {
					return ctx.Status(fiber.StatusBadRequest).JSON(ErrorResponse{"id must be provided"})
				}
				deleted, err := DB.DeleteEntry(id)
				if err != nil {
					slog.Error("failed to delete entry", "id", id, "err", err)
					return ctx.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{"failed to delete entry"})
				}
				if !deleted {
					return ctx.Status(fiber.StatusNotFound).JSON(ErrorResponse{fmt.Sprintf("no entry found with id %s", id)})
				}
				return ctx.SendStatus(fiber.StatusNoContent)
			})
		})

		// /admin/* - web endpoints for admin
		admin.Get("*.js", func(c *fiber.Ctx) error {
			filename := c.Params("*")
			content, err := jsCache.Get(filename + ".js")
			if err != nil {
				slog.Error("failed to get file from cache", "filename", filename+".js", "error", err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			finalContent := renderer.Render(content)

			c.Type(".js")
			return c.SendStream(bytes.NewBuffer(finalContent))
		})
		admin.Use(filesystem.New(filesystem.Config{
			// This should encompass: /, /login, /edit
			Root:   http.Dir(staticFilesDir),
			Browse: false,
			Index:  "index.html",

			// TODO: 404 page?
		}))
	})

	slog.Info("listening for requests", "port", port)
	err = app.Listen(fmt.Sprintf(":%d", port))
	if err != nil {
		// TODO: do we get this error if it fails to initialize or if it just fails?
		slog.Error("failed to initialize HTTP server",
			"err", err)
		return 1
	}
	return 0
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `
passd [-h|--help] [generate-key|run]

GLOBAL FLAGS:
    -h|--help                  Display this message and exit

COMMANDS:
    run                        (default) Run the passd service
    generate-key <path>        Generate a new password encryption key at <path>

ENVIRONMENT VARIABLES:
    passd supports several environment variables for controlling the behavior
    of the service.

    PASSD_AUTH_PROVIDER_URL    (required) Base URL of the authorization provider
    PASSD_REDIRECT_URL         (required) Post-authentication redirect URL
    PASSD_ALLOWED_ORIGINS      (optional) Allowed CORS origins (default: '*")
    PASSD_DISABLE_AUTH         (optional) If any value is provided, disables authentication. DO NOT USE IN PRODUCTION! (default: '')
    PASSD_PORT                 (optional) Port from which API should be served (default: %d)
    PASSD_DB_PATH              (optional) Path to the passd SQLite database (default: '%s')
    PASSD_KEY_PATH             (optional) Path to the passd password encryption key (default: '%s')
`,
		DefaultPort,
		filepath.Join(DefaultPassdDirectory, DefaultPassdDatabaseName),
		filepath.Join(DefaultPassdDirectory, DefaultPassdKeyFileName))
}

type LoginState struct {
	CameFrom string `json:"came_from"`
}

type ValidatePasswordRequest struct {
	Id       string `json:"id" xml:"name" form:"name"`
	Password string `json:"password" xml:"password" form:"password"`
}

type ValidatePasswordResponse struct {
	Result bool `json:"result"`
}

type UpsertPasswordRequest struct {
	Password string `json:"password" xml:"password" form:"password"`
}

type GetPasswordResponse struct {
	Id       string `json:"id"`
	Password string `json:"password"`
}

type GetPasswordEntryResponse struct {
	Id string `json:"id"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}
