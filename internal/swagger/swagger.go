package swagger

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/MergeMinds/mm-backend-go/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
	"github.com/wI2L/fizz/openapi"
	"go.uber.org/zap"
)

// Register routes for serving OpenAPI specification and Swagger UI.
func RegisterDocRoutes(f *fizz.Fizz, cfg *config.Config) {
	registerOpenAPI(f, cfg)
	registerSwaggerUI(f)

	if errs := f.Errors(); len(errs) > 0 {
		for _, err := range errs {
			zap.S().
				Errorw("Error while setting up API documentation", zap.Error(err))
		}
	}
}

func registerOpenAPI(f *fizz.Fizz, cfg *config.Config) {
	info := &openapi.Info{
		Title:       cfg.DocsTitle,
		Description: cfg.DocsDescription,
	}
	f.GET("/openapi.json", nil, f.OpenAPI(info, "json"))
	f.GET("/openapi.yaml", nil, f.OpenAPI(info, "yaml"))
}

//go:embed swagger-ui-dist
var swaggerAssets embed.FS

func registerSwaggerUI(f *fizz.Fizz) {
	files, err := fs.Sub(swaggerAssets, "swagger-ui-dist")
	if err != nil {
		zap.S().
			Errorw("Error while serving Swagger UI", zap.Error(err))
		return
	}

	f.Engine().StaticFS("/docs", http.FS(files))
}

// Create a new [fizz.Fizz] instance from the given [gin.Engine].
func NewFizzEngine(cfg *config.Config) *fizz.Fizz {
	// debug or release
	gin.SetMode(cfg.GinMode)

	// the hook that will intercept and handle all API errors
	tonic.SetErrorHook(apierr.ErrorHook)

	e := gin.New()

	// disable trusted proxy warning
	if err := e.SetTrustedProxies(nil); err != nil {
		zap.S().Fatal(
			"Failed to configure trusted proxies settings",
		)
	}

	f := fizz.NewFromEngine(e)
	gen := f.Generator()
	gen.SetServers([]*openapi.Server{
		{
			Description: "Local deployment",
			URL:         fmt.Sprintf("http://localhost:%d", cfg.Port),
		},
	})

	return f
}
