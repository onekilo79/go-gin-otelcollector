package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/mcarr-and/go-gin-otelcollector/album-store/model"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	// Run the other tests
	os.Exit(m.Run())
}

func setupTestRouter() (*httptest.ResponseRecorder, *tracetest.SpanRecorder, *gin.Engine) {
	spanRecorder := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)))

	// Example of how to use the in-memory repository
	repo := inMemoryRepo{
		albums: []model.Album{
			{ID: 1, Title: "Blue Train", Artist: "John Coltrane", Price: 56.99},
			{ID: 2, Title: "Jeru", Artist: "Gerry Mulligan", Price: 17.99},
			{ID: 3, Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Price: 39.99},
		},
	}
	router := setupRouter(repo, zerolog.Logger{})
	testRecorder := httptest.NewRecorder()
	router.Use(otelgin.Middleware("test-otel"))
	return testRecorder, spanRecorder, router
}

func makeKeyMap(attributes []attribute.KeyValue) map[attribute.Key]attribute.Value {
	var attributeMap = make(map[attribute.Key]attribute.Value)
	for _, keyValue := range attributes {
		attributeMap[keyValue.Key] = keyValue.Value
	}
	return attributeMap
}

func Test_getAllAlbums(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()

	var albums []model.Album

	req := httptest.NewRequest(http.MethodGet, "/albums", nil)
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &albums); err != nil {
		assert.Fail(t, "json unmarshal fail", "should be []Albums ", albums)
	}

	assert.Equal(t, http.StatusOK, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "200", attributeMap["album-store.response.code"].Emit())

	// assert on albums as needed
}

func Test_getAlbumById(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()

	var album model.Album

	req := httptest.NewRequest(http.MethodGet, "/albums/2", nil)
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &album); err != nil {
		assert.Fail(t, "json unmarshal fail", "Should be Album ", testRecorder.Body.String())
	}

	assert.Equal(t, http.StatusOK, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "200", attributeMap["album-store.response.code"].Emit())
	assert.Equal(t, `{"id":2,"title":"Jeru","artist":"Gerry Mulligan","price":17.99}`, attributeMap["album-store.response.body"].Emit())

	// assert on album as needed
}

func Test_getAlbumById_InvalidID_Character(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()

	var serverError model.ServerError

	req := httptest.NewRequest(http.MethodGet, "/albums/X", nil)
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &serverError); err != nil {
		assert.Fail(t, "json unmarshal fail", "Should be ServerError ", testRecorder.Body.String())
	}

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	expectedErrorMessage := "Album [X] not found, invalid request"

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, expectedErrorMessage, finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, expectedErrorMessage, finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["album-store.response.code"].Emit())

	assert.Equal(t, expectedErrorMessage, serverError.Message)
}

func Test_getAlbumById_NotFound(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()

	var serverError model.ServerError
	invalidAlbumID := -1666

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s%v", "/albums/", invalidAlbumID), nil)
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &serverError); err != nil {
		assert.Fail(t, "json unmarshalling fail", "Should be ServerError ", testRecorder.Body.String())
	}

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	expectedErrorMessage := fmt.Sprintf("Album [%v] not found", invalidAlbumID)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, expectedErrorMessage, finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, expectedErrorMessage, finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["album-store.response.code"].Emit())

	assert.Equal(t, expectedErrorMessage, serverError.Message)
}

func Test_postAlbum(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	var album model.Album

	expectedAlbum := model.Album{ID: 10, Title: "The Ozzman Cometh", Artist: "Black Sabbath", Price: 66.60}
	albumBody := `{"id":10,"title":"The Ozzman Cometh","artist":"Black Sabbath","price":66.6}`

	req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(albumBody))
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &album); err != nil {
		assert.Fail(t, "json unmarshalling fail", "Should be a valid Album ", testRecorder.Body.String())
	}

	assert.Equal(t, http.StatusCreated, testRecorder.Code)
	assert.Equal(t, albumBody, testRecorder.Body.String())

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))

	// attributeMap := makeKeyMap(finishedSpans[0].Attributes())

	// assert.Equal(t, "201", attributeMap["album-store.response.code"].Emit())

	assert.Equal(t, album, expectedAlbum)
	// assert on repo state as needed
}

func Test_postAlbum_BadRequest_BadJSON_MissingValues(t *testing.T) {
	// repo reset not needed
	testRecorder, spanRecorder, router := setupTestRouter()

	var serverError model.ServerError
	album := `{"xid": 10, "titlex": "Blue Train", "artistx": "Lead Belly", "pricex": 56.99, "X": "asdf"}`
	bindingErrorMessage := `{"errors":[{"field":"id","message":"below minimum value"},{"field":"title","message":"required field"},{"field":"artist","message":"required field"},{"field":"price","message":"required field"}],"message":"Album JSON field validation failed"}`

	req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(album))
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &serverError); err != nil {
		var ve validator.ValidationErrors
		errors.As(err, &ve)
		assert.Fail(t, "json unmarshalling fail", "should be ServerError ", ve.Error(), testRecorder.Body.String())
	}

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)
	assert.Equal(t, bindingErrorMessage, testRecorder.Body.String())

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "Album JSON field validation failed", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, bindingErrorMessage, finishedSpans[0].Events()[0].Name)

	assert.Equal(t, 4, len(serverError.BindingErrors))
	assert.Equal(t, "title", serverError.BindingErrors[1].Field)
	assert.Equal(t, "required field", serverError.BindingErrors[1].Message)
	assert.Equal(t, "artist", serverError.BindingErrors[2].Field)
	assert.Equal(t, "required field", serverError.BindingErrors[2].Message)
	assert.Equal(t, "price", serverError.BindingErrors[3].Field)
	assert.Equal(t, "required field", serverError.BindingErrors[3].Message)
}

func Test_postAlbum_BadRequest_BadJSON_MinValues(t *testing.T) {
	// repo reset not needed
	testRecorder, spanRecorder, router := setupTestRouter()

	album := `{"id": -1, "title": "a", "artist": "z", "price": -0.1}`
	bindingErrorMessage := `{"errors":[{"field":"id","message":"below minimum value"},{"field":"title","message":"below minimum value"},{"field":"artist","message":"below minimum value"},{"field":"price","message":"below minimum value"}],"message":"Album JSON field validation failed"}`
	var serverError model.ServerError

	req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(album))
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &serverError); err != nil {
		var ve validator.ValidationErrors
		errors.As(err, &ve)
		assert.Fail(t, "json unmarshalling fail", "should be ServerError ", ve.Error(), testRecorder.Body.String())
	}

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)
	assert.Equal(t, bindingErrorMessage, testRecorder.Body.String())

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "Album JSON field validation failed", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, bindingErrorMessage, finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["album-store.response.code"].Emit())

	assert.Equal(t, 4, len(serverError.BindingErrors))
	assert.Equal(t, "id", serverError.BindingErrors[0].Field)
	assert.Equal(t, "below minimum value", serverError.BindingErrors[0].Message)
	assert.Equal(t, "title", serverError.BindingErrors[1].Field)
	assert.Equal(t, "below minimum value", serverError.BindingErrors[1].Message)
	assert.Equal(t, "artist", serverError.BindingErrors[2].Field)
	assert.Equal(t, "below minimum value", serverError.BindingErrors[2].Message)
	assert.Equal(t, "price", serverError.BindingErrors[3].Field)
	assert.Equal(t, "below minimum value", serverError.BindingErrors[3].Message)

	// assert on repo state as needed
}

func Test_postAlbum_BadRequest_BadJSON_MaxValues(t *testing.T) {
	// repo reset not needed
	testRecorder, spanRecorder, router := setupTestRouter()

	album := `{"id": 50000000, "title": "aa", "artist": "zz", "price": 20000.00}`
	bindingErrorMessage := `{"errors":[{"field":"id","message":"above maximum value"},{"field":"price","message":"above maximum value"}],"message":"Album JSON field validation failed"}`
	var serverError model.ServerError

	req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(album))
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &serverError); err != nil {
		var ve validator.ValidationErrors
		errors.As(err, &ve)
		assert.Fail(t, "json unmarshalling fail", "should be ServerError ", ve.Error(), testRecorder.Body.String())
	}

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)
	assert.Equal(t, bindingErrorMessage, testRecorder.Body.String())

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "Album JSON field validation failed", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, bindingErrorMessage, finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["album-store.response.code"].Emit())
	assert.Equal(t, 2, len(serverError.BindingErrors))
	assert.Equal(t, "id", serverError.BindingErrors[0].Field)
	assert.Equal(t, "above maximum value", serverError.BindingErrors[0].Message)
	assert.Equal(t, "price", serverError.BindingErrors[1].Field)
	assert.Equal(t, "above maximum value", serverError.BindingErrors[1].Message)

	// assert on repo state as needed
}

func Test_postAlbum_BadRequest_Malformed_JSON(t *testing.T) {
	// repo reset not needed
	testRecorder, spanRecorder, router := setupTestRouter()

	var serverError model.ServerError
	requestBody := `{"id": -1,`

	req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(requestBody))
	router.ServeHTTP(testRecorder, req)
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &serverError); err != nil {
		assert.Fail(t, "", "should be ServerError ")
	}

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "Malformed JSON. Not valid for Album", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "Malformed JSON. unexpected EOF", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["album-store.response.code"].Emit())
	assert.Equal(t, "Malformed JSON. Not valid for Album", serverError.Message)
	assert.Equal(t, 0, len(serverError.BindingErrors))

	// assert on repo state as needed
}

func Test_getSwagger(t *testing.T) {
	// repo reset not needed
	testRecorder, _, router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	router.ServeHTTP(testRecorder, req)
	bodyString := testRecorder.Body.String()

	assert.Equal(t, http.StatusOK, testRecorder.Code)
	assert.Contains(t, bodyString, "swagger-ui.css")
}

func Test_getStatus(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	router.ServeHTTP(testRecorder, req)

	responseBodyString := testRecorder.Body.String()

	assert.Equal(t, http.StatusOK, testRecorder.Code)
	assert.Equal(t, `{"status":"OK"}`, responseBodyString)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))
}

func Test_getMetrics(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(testRecorder, req)

	responseBodyString := testRecorder.Body.String()

	assert.Equal(t, http.StatusOK, testRecorder.Code)
	assert.Contains(t, responseBodyString, `go_gc_duration_seconds`)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))
}

func Benchmark_getAllAlbums(b *testing.B) {
	testRecorder, _, router := setupTestRouter()

	var albums []model.Album
	req := httptest.NewRequest(http.MethodGet, "/albums", nil)

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(testRecorder, req)
		if err := json.Unmarshal(testRecorder.Body.Bytes(), &albums); err != nil {
			assert.Fail(b, "json unmarshalling fail", "should be []Album ", testRecorder.Body.String())
		}
		testRecorder.Body.Reset() //get requests need resets else the returned body is concatenated
	}
}

func Benchmark_getAlbumById(b *testing.B) {
	testRecorder, _, router := setupTestRouter()

	var album model.Album
	req := httptest.NewRequest(http.MethodGet, "/albums/2", nil)

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(testRecorder, req)
		if err := json.Unmarshal(testRecorder.Body.Bytes(), &album); err != nil {
			assert.Fail(b, "json unmarshalling fail", "should be Album ", testRecorder.Body.String())
		}
		testRecorder.Body.Reset() //get requests need resets else the returned body is concatenated
	}
}

func Benchmark_getAlbumById_BadRequest(b *testing.B) {
	testRecorder, _, router := setupTestRouter()

	var serverError model.ServerError
	req := httptest.NewRequest(http.MethodGet, "/albums/5666", nil)

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(testRecorder, req)
		if err := json.Unmarshal(testRecorder.Body.Bytes(), &serverError); err != nil {
			assert.Fail(b, "json unmarshalling fail", "should be ServerError ", testRecorder.Body.String())
		}
		testRecorder.Body.Reset() //get requests need resets else the returned body is concatenated
	}
}

func Benchmark_postAlbum(b *testing.B) {
	testRecorder, _, router := setupTestRouter()

	var albumReturned model.Album
	albumJson := `{"id": "10", "title": "The Ozzman Cometh", "artist": "Black Sabbath", "price": 56.99}`
	req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(albumJson))

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(testRecorder, req)
		if err := json.Unmarshal(testRecorder.Body.Bytes(), &albumReturned); err != nil {
			assert.Fail(b, "json unmarshalling fail", "should be Album ", testRecorder.Body.String())
		}
		testRecorder.Body.Reset()
	}
}

func Benchmark_postAlbum_BadRequest_BadJson(b *testing.B) {
	testRecorder, _, router := setupTestRouter()

	var returnedError model.ServerError
	albumJson := `{"xid": "10", "titlex": "Blue Train", "artistx": "John Coltrane", "pricex": 56.99, "X": "asdf"}`
	req := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(albumJson))

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(testRecorder, req)
		if err := json.Unmarshal(testRecorder.Body.Bytes(), &returnedError); err != nil {
			assert.Fail(b, "json unmarshalling fail", "Should be ServerError ", testRecorder.Body.String())
		}
		testRecorder.Body.Reset()
	}
}
