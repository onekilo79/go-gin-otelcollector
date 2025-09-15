package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcarr-and/go-gin-otelcollector/proxy-service/model"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type OtelHttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	DefaultClient OtelHttpClient
)

func init() {
	DefaultClient = otelhttp.DefaultClient
}

// @title           Proxy Service API
// @version         1.0
// @description     Simple golang application that proxies calls to Album-Store
// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
// @host      localhost:9070
// @BasePath /
// @schemes http

// GetAlbums godoc
// @Summary Get all Albums
// @Schemes
// @Description get all the albums in the store
// @Tags albums
// @Produce json
// @Success 200 {array} model.Album
// @Failure 500 {object} model.ServerError
// @Router /albums [get]
func makeGetAlbumsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/albums GET")
		defer span.End()
		// proxy call to album-Store
		resp, err := Get(c.Request.Context(), albumStoreURL+"/albums")
		setResponseCodeIfPresent(resp, span)
		if handleResponseHasError(c, err, "getAlbums", span) {
			return
		}
		albumStoreResponseBodyJson, failed := processResponseBody(c, span, resp.Body)
		if failed {
			return
		}
		if handleResponseCodeHasError(c, resp.StatusCode, "getAlbums", span) {
			return
		}
		span.SetAttributes(attribute.Key("proxy-service.response.code").Int(http.StatusOK))
		span.SetStatus(codes.Ok, "")
		c.JSON(http.StatusOK, albumStoreResponseBodyJson)
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
// @Failure 500 {object} model.ServerError
// @Router /albums/{id} [get]
func makeGetAlbumByIdHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/albums/:id GET")
		defer span.End()
		id := c.Param("id")
		span.SetAttributes(attribute.Key("proxy-service.request.parameters").String(fmt.Sprintf("%s=%s", "ID", id)))
		albumID, err := strconv.Atoi(id)
		// param ID is expected to be a number so fail if cannot covert to integer
		if buildErrorInvalidRequestParameters(c, err, id, span) {
			return
		}
		// proxy call to album-Store
		resp, err := Get(c.Request.Context(), fmt.Sprintf("%v/albums/%v", albumStoreURL, albumID))
		setResponseCodeIfPresent(resp, span)
		if handleResponseHasError(c, err, "getAlbumById", span) {
			return
		}
		albumStoreResponseBodyJson, failed := processResponseBody(c, span, resp.Body)
		if failed {
			return
		}
		if handleResponseCodeHasError(c, resp.StatusCode, "getAlbumById", span) {
			return
		}
		span.SetAttributes(attribute.Key("proxy-service.response.code").Int(http.StatusOK))
		span.SetStatus(codes.Ok, "")
		c.JSON(http.StatusOK, albumStoreResponseBodyJson)
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
// @Failure 500 {object} model.ServerError
// @Router /albums [post]
func makePostAlbumsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetName("/albums POST")
		defer span.End()
		requestBodyString, failed := processRequestBody(c, span, c.Request.Body)
		if failed {
			return
		}
		// proxy call to album-Store
		resp, err := Post(c.Request.Context(), albumStoreURL+"/albums", "application/json", strings.NewReader(fmt.Sprintf("%v", requestBodyString)))
		setResponseCodeIfPresent(resp, span)
		if handleResponseHasError(c, err, "postAlbum", span) {
			return
		}
		albumStoreResponseBodyJson, failed := processResponseBody(c, span, resp.Body)
		if failed {
			return
		}
		if handleResponseCodeHasError(c, resp.StatusCode, "postAlbum", span) {
			return
		}
		span.SetAttributes(attribute.Key("proxy-service.response.code").Int(http.StatusCreated))
		span.SetStatus(codes.Ok, "")
		c.JSON(http.StatusCreated, albumStoreResponseBodyJson)
	}
}

func setResponseCodeIfPresent(resp *http.Response, span trace.Span) {
	if resp != nil {
		span.SetAttributes(attribute.Key("album-store.response.code").Int(resp.StatusCode))
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

func processResponseBody(c *gin.Context, span trace.Span, body io.ReadCloser) (interface{}, bool) {
	var jsonBody interface{}
	byteArray, _ := io.ReadAll(body)
	jsonBodyString := string(byteArray[:])
	var err = json.NewDecoder(strings.NewReader(jsonBodyString)).Decode(&jsonBody)

	if err != nil {
		buildMalformedResponseJsonErrorResponse(c, span, jsonBodyString, "error from album-store Malformed JSON returned", http.StatusInternalServerError)
		return jsonBody, true
	}

	err = body.Close()
	if err != nil {
		errorMessage := fmt.Sprintf("error album-store closing response %v", err)
		span.AddEvent(errorMessage)
		span.SetStatus(codes.Error, errorMessage)
		span.SetAttributes(attribute.Key("http.response.code").Int(http.StatusInternalServerError))
		c.AbortWithStatusJSON(http.StatusInternalServerError, model.ServerError{Message: errorMessage})
		return jsonBody, true
	}
	span.SetAttributes(attribute.Key("album-store.response.body").String(jsonBodyString))
	span.SetAttributes(attribute.Key("proxy-service.response.body").String(jsonBodyString))
	return jsonBody, false
}

func processRequestBody(c *gin.Context, span trace.Span, reader io.ReadCloser) (string, bool) {
	var requestBody interface{}
	byteArray, _ := io.ReadAll(reader)
	jsonBodyString := string(byteArray[:])
	var err = json.NewDecoder(strings.NewReader(jsonBodyString)).Decode(&requestBody)
	span.SetAttributes(attribute.Key("proxy-service.request.body").String(jsonBodyString))

	if err != nil {
		errorMessage := fmt.Sprintf("invalid request json body %v", jsonBodyString)
		buildMalformedRequestJsonErrorResponse(c, span, jsonBodyString, errorMessage, http.StatusBadRequest)
		span.SetAttributes(attribute.Key("proxy-service.response.body").String(fmt.Sprintf("{\"message\":\"%v\"}", errorMessage)))
		return jsonBodyString, true
	}
	return jsonBodyString, false
}

func buildMalformedRequestJsonErrorResponse(c *gin.Context, span trace.Span, response string, errorMessage string, errorCode int) bool {
	span.SetStatus(codes.Error, errorMessage)
	span.AddEvent(errorMessage)
	span.SetAttributes(attribute.Key("proxy-service.request.body").String(response))
	span.SetAttributes(attribute.Key("proxy-service.response.code").Int(errorCode))
	c.AbortWithStatusJSON(errorCode, model.ServerError{Message: errorMessage})
	return true
}

func buildMalformedResponseJsonErrorResponse(c *gin.Context, span trace.Span, response string, errorMessage string, errorCode int) bool {
	span.SetStatus(codes.Error, errorMessage)
	span.AddEvent(errorMessage)
	span.SetAttributes(attribute.Key("album-store.response.body").String(response))
	span.SetAttributes(attribute.Key("proxy-service.response.body").String(fmt.Sprintf(`{"message":"%v"}`, errorMessage)))
	span.SetAttributes(attribute.Key("proxy-service.response.code").Int(errorCode))
	c.AbortWithStatusJSON(errorCode, model.ServerError{Message: errorMessage})
	return true
}

func handleResponseHasError(c *gin.Context, err error, methodName string, span trace.Span) bool {
	if err != nil {
		errorMessage := fmt.Sprintf("error contacting album-store %s %v", methodName, err)
		span.AddEvent(errorMessage)
		span.SetStatus(codes.Error, errorMessage)
		span.SetAttributes(attribute.Key("proxy-service.response.body").String(fmt.Sprintf(`{"message":"%v"}`, errorMessage)))
		span.SetAttributes(attribute.Key("proxy-service.response.code").Int(http.StatusInternalServerError))
		c.AbortWithStatusJSON(http.StatusInternalServerError, model.ServerError{Message: errorMessage})
		return true
	}
	return false
}

func handleResponseCodeHasError(c *gin.Context, responseCode int, methodName string, span trace.Span) bool {
	if responseCode != http.StatusOK && responseCode != http.StatusCreated {
		errorMessage := fmt.Sprintf("album-store returned error %s", methodName)
		span.AddEvent(errorMessage)
		span.SetStatus(codes.Error, errorMessage)
		span.SetAttributes(attribute.Key("proxy-service.response.code").Int(responseCode))
		c.AbortWithStatusJSON(responseCode, model.ServerError{Message: errorMessage})
		return true
	}
	return false
}

func buildErrorInvalidRequestParameters(c *gin.Context, err error, id string, span trace.Span) bool {
	if err != nil {
		errorMessage := fmt.Sprintf("%s [%s] %s", "error invalid ID", id, "requested")
		span.SetStatus(codes.Error, errorMessage)
		span.AddEvent(errorMessage)
		span.SetAttributes(attribute.Key("proxy-service.response.code").Int(http.StatusBadRequest))
		span.SetAttributes(attribute.Key("proxy-service.response.body").String(fmt.Sprintf(`{"message":"%v"}`, errorMessage)))
		c.AbortWithStatusJSON(http.StatusBadRequest, model.ServerError{Message: errorMessage})
		return true
	}
	return false
}

func setupRouter() *gin.Engine {
	router := gin.Default()
	router.Use(otelgin.Middleware(serviceName)) // add OpenTelemetry to Gin
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/albums", makeGetAlbumsHandler())
	router.GET("/albums/:id", makeGetAlbumByIdHandler())
	router.POST("/albums", makePostAlbumsHandler())
	router.GET("/status", makeStatusHandler())
	router.GET("/metrics", makeMetricsHandler())
	return router
}

const (
	serviceName  = "proxy-service"
	startAddress = "0.0.0.0:9070"
)

var version = "No-Version"
var gitHash = "No-Hash"
var albumStoreURL = "http://localhost:9080"

func main() {
	proxyLog := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logInfo := zerolog.New(os.Stdout).With().Timestamp().Logger()

	logInfo.Info().Msg(fmt.Sprintf("version: %v-%v", version, gitHash))
	shutdownTraceProvider, err := initOtelProvider(serviceName, version, gitHash, proxyLog)
	if err != nil {
		proxyLog.Err(err)
	}

	albumStoreUrlEnv := os.Getenv("ALBUM_STORE_URL")
	if albumStoreURL != "" {
		albumStoreURL = albumStoreUrlEnv
	}

	router := setupRouter()
	//serve requests until termination signal is sent.

	srv := &http.Server{
		Addr:    startAddress,
		Handler: h2c.NewHandler(router, &http2.Server{}),
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			proxyLog.Err(err)
		}
	}()
	<-sigChan

	logInfo.Info().Msg("Server shutdown with 500ms timeout...")
	ctxServer, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	logInfo.Info().Msg("OpenTelemetry TraceProvider flushing & shutting down")
	if err := shutdownTraceProvider(ctxServer); err != nil {
		proxyLog.Err(err)
	}
	logInfo.Info().Msg("OpenTelemetry TraceProvider exited")

	if err := srv.Shutdown(ctxServer); err != nil {
		proxyLog.Err(err)
	}
	<-ctxServer.Done()

	logInfo.Info().Msg("Server exiting")
}

// Extracted methods from https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/instrumentation/net/http/otelhttp/client.go v0.37.0
// this is to allow use of interface for httpClient and be able to mock out responses

func Get(ctx context.Context, targetURL string) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	return DefaultClient.Do(req)
}

// Post is a convenient replacement for http.Post that adds a span around the request.
func Post(ctx context.Context, targetURL, contentType string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return DefaultClient.Do(req)
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
