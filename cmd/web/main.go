package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/labstack/echo/v4"
	otelecho "go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

	otelx "github.com/thangchung/go-coffeeshop/pkg/otel"
)

//go:embed app
var embededFiles embed.FS

func getFileSystem(useOS bool) http.FileSystem {
	if useOS {
		log.Print("using live mode")

		return http.FS(os.DirFS("app"))
	}

	log.Print("using embed mode")

	fsys, err := fs.Sub(embededFiles, "app")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

type UrlModel struct {
	Url string `json:"url"`
}

func main() {
	ctx := context.Background()

	reverseProxyURL, ok := os.LookupEnv("REVERSE_PROXY_URL")
	if !ok || reverseProxyURL == "" {
		glog.Fatalf("web: environment variable not declared: reverseProxyURL")
	}

	webPort, ok := os.LookupEnv("WEB_PORT")
	if !ok || webPort == "" {
		glog.Fatalf("web: environment variable not declared: webPort")
	}

	e := echo.New()

	// OpenTelemetry init
	serviceName := os.Getenv("APP_NAME")
	if serviceName == "" {
		serviceName = "web"
	}
	serviceVersion := os.Getenv("APP_VERSION")
	if serviceVersion == "" {
		serviceVersion = "dev"
	}
	shutdown, metricsHandler, err := otelx.Setup(ctx, serviceName, serviceVersion)
	if err != nil {
		glog.Fatalf("OpenTelemetry init error: %s", err)
	}
	defer func() {
		_ = shutdown(context.Background())
	}()
	e.Use(otelecho.Middleware(serviceName))

	useOS := len(os.Args) > 1 && os.Args[1] == "live"
	assetHandler := http.FileServer(getFileSystem(useOS))
	e.GET("/", echo.WrapHandler(assetHandler))
	e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", assetHandler)))
	e.GET("/metrics", echo.WrapHandler(metricsHandler))
	e.GET("/reverse-proxy-url", func(c echo.Context) error {
		return c.JSON(http.StatusOK, UrlModel{Url: reverseProxyURL})
	})

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%v", webPort)))
}
