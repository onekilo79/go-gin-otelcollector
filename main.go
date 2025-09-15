package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"
	"time"

	_ "github.com/mcarr-and/go-gin-otelcollector/album-store/api"
	"github.com/mcarr-and/go-gin-otelcollector/album-store/model"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// @title           Album Store API
// @version         1.0
// @description     Simple golang album store CRUD application
// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
// @host      localhost:9080
// @BasePath /

// GetAlbums godoc
// @Summary Get all Albums
// @Schemes
// @Description get all the albums in the store
// @Tags albums
// @Produce json
// @Success 200 {array} model.Album
// @Router /albums [get]
func makeGetAlbumsHandler(repo AlbumRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/albums GET")
		defer span.End()
		span.SetStatus(codes.Ok, "")
		span.SetAttributes(attribute.Key("album-store.response.code").Int(http.StatusOK))
		c.JSON(http.StatusOK, repo.List())
	}
}

// GetAlbumById godoc
// @Summary Get Album by id
// @Schemes
// @Description get as single album by id
// @Tags albums
// @Param  id query int true  "int valid" minimum(1)
// @Produce json
// @Success 200 {object} model.Album
// @Failure 400 {object} model.ServerError
// @Router /albums/{id} [get]
// replaced by makeGetAlbumByIDHandler
func makeGetAlbumByIDHandler(repo AlbumRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/albums/:id GET")
		defer span.End()
		id := c.Param("id")
		span.SetAttributes(attribute.Key("album-store.request.parameters").String(fmt.Sprintf("%s=%s", "ID", id)))
		albumId, err := strconv.Atoi(id)
		if bindJsonToModelFails(c, err, id, span) {
			return
		}
		album, found := repo.GetByID(albumId)
		if found {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(attribute.Key("album-store.response.code").Int(http.StatusOK))
			jsonVal, _ := json.Marshal(album)
			span.SetAttributes(attribute.Key("album-store.response.body").String(string(jsonVal)))
			c.JSON(http.StatusOK, album)
			return
		}
		errorMessage := fmt.Sprintf("Album [%v] not found", albumId)
		serverError := model.ServerError{Message: errorMessage}
		span.SetStatus(codes.Error, serverError.Message)
		span.AddEvent(errorMessage)
		span.SetAttributes(attribute.Key("album-store.response.code").Int(http.StatusBadRequest))
		c.AbortWithStatusJSON(http.StatusBadRequest, serverError)
	}
}

// PostAlbum godoc
// @Summary Create album
// @Schemes
// @Description add a new album to the store
// @Tags albums
// @Param request body model.Album true "album"
// @Accept json
// @Produce json
// @Success 201 {object} model.Album
// @Failure 400 {object} model.ServerError
// @Router /albums [post]
func makePostAlbumHandler(repo AlbumRepository, logError zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/albums POST")
		defer span.End()

		var album model.Album
		if err := c.ShouldBindJSON(&album); err != nil {
			const errorMessage = "Album JSON field validation failed"
			span.SetStatus(codes.Error, errorMessage)
			if !processValidationBindingError(err, span, c, logError) {
				buildMalformedJsonErrorResponse(c, span, err)
			}
			return
		}
		repo.Add(album)
		span.SetStatus(codes.Ok, "")
		c.JSON(http.StatusCreated, album)
	}
}

// Status godoc
// @Summary Status of service
// @Schemes
// @Description get the status of the service
// @Tags albums
// @Produce json
// @Success 200 {string} status
// @Router /status [get]
func makeStatusHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/status")
		span.SetStatus(codes.Ok, "")
		defer span.End()
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	}
}

// Metrics godoc
// @Summary Prometheus metrics
// @Schemes
// @Description get Prometheus metrics for the service
// @Tags albums
// @Produce plain
// @Success 200 {string} metrics
// @Router /status [get]
func makeMetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/metrics")
		span.SetStatus(codes.Ok, "")
		defer span.End()
		promhttp.Handler().ServeHTTP(c.Writer, c.Request)
	}
}

type AlbumRepository interface {
	List() []model.Album
	GetByID(id int) (model.Album, bool)
	Add(album model.Album) model.Album
}

type inMemoryRepo struct {
	albums []model.Album
}

// Add implements AlbumRepository.
func (r inMemoryRepo) Add(album model.Album) model.Album {
	r.albums = append(r.albums, album)
	return album
}

func (r inMemoryRepo) List() []model.Album {
	return r.albums
}

func (r inMemoryRepo) GetByID(id int) (model.Album, bool) {
	for _, album := range r.albums {
		if album.ID == id {
			return album, true
		}
	}
	return model.Album{}, false
}

func getErrorMsg(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "required field"
	case "min":
		return "below minimum value"
	case "max":
		return "above maximum value"
	}
	return ""
}

var serviceName = "album-store"
var startAddress = "0.0.0.0:9080"

func bindJsonToModelFails(c *gin.Context, err error, id string, span trace.Span) bool {
	if err != nil {
		errorMessage := fmt.Sprintf("Album [%s] not found, invalid request", id)
		serverError := model.ServerError{Message: errorMessage}
		span.SetStatus(codes.Error, serverError.Message)
		span.AddEvent(errorMessage)
		// span.RecordError(err, )// todo - figure out when to use this instead of event
		span.SetAttributes(attribute.Key("album-store.response.code").Int(http.StatusBadRequest))
		c.AbortWithStatusJSON(http.StatusBadRequest, serverError)
		return true
	}
	return false
}

func buildMalformedJsonErrorResponse(c *gin.Context, span trace.Span, err error) bool {
	span.SetStatus(codes.Error, "Malformed JSON. Not valid for Album")
	span.AddEvent(fmt.Sprintf("Malformed JSON. %s", err))
	span.SetAttributes(attribute.Key("album-store.response.code").Int(http.StatusBadRequest))
	c.AbortWithStatusJSON(http.StatusBadRequest, model.ServerError{Message: "Malformed JSON. Not valid for Album"})
	return true
}

func processValidationBindingError(err error, span trace.Span, c *gin.Context, logError zerolog.Logger) bool {
	var newAlbum model.Album
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		bindingErrorMessages := make([]*model.BindingErrorMsg, len(validationErrors))
		for index, fieldError := range validationErrors {
			field, _ := reflect.TypeOf(&newAlbum).Elem().FieldByName(fieldError.Field())
			fieldJSONName, okay := field.Tag.Lookup("json")
			if !okay {
				logError.Fatal().Msg(fmt.Sprintf("No json type on Struct model.Album %s Expecting : `json:\"title\" ...`", fieldError.Field()))
			}
			bindingErrorMessages[index] = &model.BindingErrorMsg{Field: fieldJSONName, Message: getErrorMsg(fieldError)}
		}
		serverError := model.ServerError{BindingErrors: bindingErrorMessages, Message: "Album JSON field validation failed"}
		serverErrorMessage, _ := json.Marshal(serverError)
		span.SetStatus(codes.Error, "Album JSON field validation failed")
		span.AddEvent(string(serverErrorMessage))
		span.SetAttributes(attribute.Key("album-store.response.code").Int(http.StatusBadRequest))
		c.AbortWithStatusJSON(http.StatusBadRequest, serverError)
		return true
	}
	return false
}

func setupRouter(repo AlbumRepository, logError zerolog.Logger) *gin.Engine {
	router := gin.New()

	router.Use(otelgin.Middleware(serviceName)) // add OpenTelemetry to Gin

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/albums", makeGetAlbumsHandler(repo))
	router.GET("/albums/:id", makeGetAlbumByIDHandler(repo))
	router.POST("/albums", makePostAlbumHandler(repo, logError))
	router.GET("/status", makeStatusHandler())
	router.GET("/metrics", makeMetricsHandler())

	return router
}

func startServer(router *gin.Engine) {
	logError := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logInfo := zerolog.New(os.Stdout).With().Timestamp().Logger()

	logInfo.Info().Msg(fmt.Sprintf("version: %v-%v", version, gitHash))
	shutdownTraceProvider, err := initOtelProvider(serviceName, version, gitHash, logInfo)
	if err != nil {
		logError.Fatal().Err(err)
	}

	// Start the server
	srv := &http.Server{
		Addr:    startAddress,
		Handler: h2c.NewHandler(router, &http2.Server{}),
	}

	// Graceful shutdown
	setupServerStopCallback(srv, logError, logInfo, shutdownTraceProvider)
}

func setupServerStopCallback(srv *http.Server, logError zerolog.Logger, logInfo zerolog.Logger, shutdownTraceProvider func(context.Context) error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logError.Fatal().Err(err)
		}
	}()
	// Wait for interrupt signal to gracefully shutdown the server
	<-sigChan

	logInfo.Info().Msg("Server shutdown with 500ms timeout...")

	fmt.Println("\nShutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	logInfo.Info().Msg("OpenTelemetry TraceProvider flushing & shutting down")
	if err := shutdownTraceProvider(ctx); err != nil {
		logError.Fatal().Err(err)
	}
	logInfo.Info().Msg("OpenTelemetry TraceProvider exited")

	if err := srv.Shutdown(ctx); err != nil {
		logError.Fatal().Err(err)
	}
	<-ctx.Done()
	logInfo.Info().Msg("Server exiting")
}

var version = "No-Version"
var gitHash = "No-Hash"

func main() {
	logError := zerolog.New(os.Stderr).With().Timestamp().Logger()
	// Example of how to use the in-memory repository
	repo := inMemoryRepo{
		albums: []model.Album{},
	}
	// Set up Gin router
	router := setupRouter(repo, logError)
	startServer(router)
}

// Set up the context for this Application in Open Telemetry
// application name, application version, k8s namespace , k8s instance name (horizontal scaling)
func setupOtelResource(serviceName string, version string, gitHash string, ctx context.Context, namespace *string, instanceName *string) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version+"-"+gitHash),
			semconv.ServiceNamespaceKey.String(*namespace),
			semconv.ServiceInstanceIDKey.String(*instanceName),
		),
	)
	return res, err
}

// InitOtelProvider - Initializes an OTLP exporter, and configures the corresponding trace and metric providers.
func initOtelProvider(serviceName string, version string, gitHash string, log zerolog.Logger) (func(context.Context) error, error) {
	ctx := context.Background()

	namespace := os.Getenv("NAMESPACE")
	instanceName := os.Getenv("INSTANCE_NAME")
	otelLocation := os.Getenv("OTEL_LOCATION")
	if instanceName == "" || otelLocation == "" || namespace == "" {
		log.Fatal().Msg(fmt.Sprintf("Env variables not assigned NAMESPACE=%v, INSTANCE_NAME=%v, OTEL_LOCATION=%v", namespace, instanceName, otelLocation))
	}

	otelResource, err := setupOtelResource(serviceName, version, gitHash, ctx, &namespace, &instanceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	otelTraceExporter, err := setupOtelHttpTrace(ctx, &otelLocation)
	if err != nil {
		return nil, err
	}

	traceProvider := setupOtelTraceProvider(otelTraceExporter, otelResource)
	return traceProvider.Shutdown, nil //return shutdown signal so the application can trigger shutting itself down
}

func setupOtelTraceProvider(traceExporter *otlptrace.Exporter, otelResource *resource.Resource) *sdktrace.TracerProvider {
	// Register the trace exporter with a TracerProvider, using a batch span processor to aggregate spans before export.
	batchSpanProcessor := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(otelResource),
		sdktrace.WithSpanProcessor(batchSpanProcessor),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{}) // set global propagator to tracecontext (the default is no-op).
	return tracerProvider
}

func setupOtelHttpTrace(ctx context.Context, otelLocation *string) (*otlptrace.Exporter, error) {
	// insecure transport here DO NOT USE IN PROD
	client := otlptracehttp.NewClient(
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(*otelLocation),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	)
	err := client.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start http client: %w", err)
	}
	traceExporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}
	return traceExporter, nil
}
