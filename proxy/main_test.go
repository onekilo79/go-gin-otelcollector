package main

import (
	"bytes"
	"errors"
	"fmt"

	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMain(m *testing.M) {
	//Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Run the other tests
	os.Exit(m.Run())
}

// StubClient stuct is to follow the HttpClient interface so it can be mocked out.
type StubClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

// Do is to be able override the OpenTelemetry DefaultHttp client which wraps the http calls and instead of making the call return a MockResponse we can control
func (m *StubClient) Do(req *http.Request) (*http.Response, error) {
	return StubResponseFunc(req)
}

var (
	// StubResponseFunc fetches the mock client's `Do` func
	StubResponseFunc func(req *http.Request) (*http.Response, error)
)

func setupTestRouter() (*httptest.ResponseRecorder, *tracetest.SpanRecorder, *gin.Engine) {
	spanRecorder := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)))
	router := setupRouter()
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

func Test_getAllAlbums_Success(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	responseBody := `[{"artist":"Black Sabbath","id":10,"price":66.6,"title":"The Ozzman Cometh"}]`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a success message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/albums", nil)
	router.ServeHTTP(testRecorder, req)
	bytesArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(bytesArr)

	assert.Equal(t, http.StatusOK, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "200", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "200", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, responseBody, returnedBody)
}

func Test_getAllAlbums_Failure_Album_Returns_Error(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	// test client setup not needed

	//inject in failure message to respond with that we could not get to the album-service
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return nil, errors.New("ERROR FROM WEB SERVER")
	}

	req := httptest.NewRequest(http.MethodGet, "/albums", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusInternalServerError, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "error contacting album-service getAlbums ERROR FROM WEB SERVER", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "error contacting album-service getAlbums ERROR FROM WEB SERVER", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "500", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"error contacting album-service getAlbums ERROR FROM WEB SERVER"}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "unknown", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, "unknown", attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"error contacting album-service getAlbums ERROR FROM WEB SERVER"}`, returnedBody)
}

func Test_getAllAlbums_Failure_Malformed_Response(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	//inject in failure message to respond with that we could not get to the album-service
	responseBody := `[{"artist":"Black Sabbath","id":10,"price":66.6`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a failure message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/albums", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusInternalServerError, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "error from album-service Malformed JSON returned", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "error from album-service Malformed JSON returned", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "500", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"error from album-service Malformed JSON returned"}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "200", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"error from album-service Malformed JSON returned"}`, returnedBody)
}

func Test_getAllAlbums_Failure_Bad_Request(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	responseBody := `{"Wu-Tang":"is for the children"}`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a bad request response code and message from the server
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/albums", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "album-service returned error getAlbums", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "album-service returned error getAlbums", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "400", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"album-service returned error getAlbums"}`, returnedBody)
}

func Test_getAlbumById_Success(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	responseBody := `{"artist":"Black Sabbath","id":10,"price":66.6,"title":"The Ozzman Cometh"}`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a success message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/albums/1", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusOK, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "200", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "200", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, responseBody, returnedBody)
}

func Test_getAlbumById_Failure_Bad_Request(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	responseBody := `{"Wu-Tang":"is for the children"}`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a success message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/albums/1", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "album-service returned error getAlbumById", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "album-service returned error getAlbumById", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "400", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"album-service returned error getAlbumById"}`, returnedBody)
}

func Test_getAlbumById_Failure_Album_Returns_Error(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	//inject in failure message to respond with that we could not get to the album-service
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return nil, errors.New("ERROR FROM WEB SERVER")
	}

	req := httptest.NewRequest(http.MethodGet, "/albums/1", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusInternalServerError, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "error contacting album-service getAlbumById ERROR FROM WEB SERVER", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "error contacting album-service getAlbumById ERROR FROM WEB SERVER", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "500", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"error contacting album-service getAlbumById ERROR FROM WEB SERVER"}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "unknown", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, "unknown", attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"error contacting album-service getAlbumById ERROR FROM WEB SERVER"}`, returnedBody)
}

func Test_getAlbumById_Failure_Album_BadId(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	//inject in failure message to respond with that we could not get to the album-service
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/albums/X", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "error invalid ID [X] requested", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "error invalid ID [X] requested", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "400", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"error invalid ID [X] requested"}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "unknown", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, "unknown", attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"error invalid ID [X] requested"}`, returnedBody)
}

func Test_getAlbumById_Failure_Malformed_Response(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	//inject in failure message to respond with that we could not get to the album-service
	responseBody := `{"artist":"Black Sabbath","id":10,"price":66.6`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a failure message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/albums/1", nil)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusInternalServerError, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "error from album-service Malformed JSON returned", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "error from album-service Malformed JSON returned", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "500", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"error from album-service Malformed JSON returned"}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "200", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"error from album-service Malformed JSON returned"}`, returnedBody)
}

func Test_postAlbums_Success(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	requestBody := `{"artist":"Black Sabbath","id":10,"price":66.6,"title":"The Ozzman Cometh"}`
	requestBodyReader := io.NopCloser(bytes.NewReader([]byte(requestBody)))

	responseBody := `{"artist":"Black Sabbath","id":10,"price":66.6,"title":"The Ozzman Cometh"}`
	responseBodyReader := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a success message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       responseBodyReader,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/albums", requestBodyReader)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusCreated, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Ok, finishedSpans[0].Status().Code)
	assert.Equal(t, "", finishedSpans[0].Status().Description)

	assert.Equal(t, 0, len(finishedSpans[0].Events()))

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, requestBody, attributeMap["proxy-service.request.body"].Emit())

	assert.Equal(t, "201", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "201", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, responseBody, returnedBody)
}

func Test_postAlbums_Failure_Album_Empty_Request_Body(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	requestBody := ``
	requestBodyReader := io.NopCloser(bytes.NewReader([]byte(requestBody)))

	responseBody := `{"errors":null,"message":"invalid request json body "}`

	//Mock not used so setup as ignored
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/albums", requestBodyReader)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "invalid request json body ", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "invalid request json body ", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, "", attributeMap["proxy-service.request.body"].Emit())

	assert.Equal(t, "400", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"invalid request json body "}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "unknown", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, "unknown", attributeMap["album-service.response.body"].Emit())
	assert.Equal(t, responseBody, returnedBody)
}

func Test_postAlbums_Failure_Album_Malformed_Request_Body(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	requestBody := `{"title":"Ozzman Cometh"`
	requestBodyReader := io.NopCloser(bytes.NewReader([]byte(requestBody)))

	//Mock not used so setup as ignored
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/albums", requestBodyReader)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, fmt.Sprintf("invalid request json body %v", requestBody), finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, fmt.Sprintf("invalid request json body %v", requestBody), finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, requestBody, attributeMap["proxy-service.request.body"].Emit())

	assert.Equal(t, "400", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"invalid request json body {"title":"Ozzman Cometh""}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "unknown", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, "unknown", attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"invalid request json body {\"title\":\"Ozzman Cometh\""}`, returnedBody)
}

func Test_postAlbums_Failure_Album_Returns_Error(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	requestBody := `{"artist":"Black Sabbath","id":10,"price":66.6,"title":"The Ozzman Cometh"}`
	requestBodyReader := io.NopCloser(bytes.NewReader([]byte(requestBody)))

	//inject in failure message to respond with that we could not get to the album-service
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return nil, errors.New("ERROR FROM WEB SERVER")
	}

	req := httptest.NewRequest(http.MethodPost, "/albums", requestBodyReader)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusInternalServerError, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "error contacting album-service postAlbum ERROR FROM WEB SERVER", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "error contacting album-service postAlbum ERROR FROM WEB SERVER", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, requestBody, attributeMap["proxy-service.request.body"].Emit())

	assert.Equal(t, "500", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"error contacting album-service postAlbum ERROR FROM WEB SERVER"}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "unknown", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, "unknown", attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"error contacting album-service postAlbum ERROR FROM WEB SERVER"}`, returnedBody)
}

func Test_postAlbums_Failure_Malformed_Response(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	requestBody := `{"artist":"Black Sabbath","id":10,"price":66.6,"title":"The Ozzman Cometh"}`
	requestBodyReader := io.NopCloser(bytes.NewReader([]byte(requestBody)))

	//inject in failure message to respond with that we could not get to the album-service
	responseBody := `[{"artist":"Black Sabbath","id":10,"price":66.6`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a failure message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/albums", requestBodyReader)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusInternalServerError, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "error from album-service Malformed JSON returned", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "error from album-service Malformed JSON returned", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, requestBody, attributeMap["proxy-service.request.body"].Emit())

	assert.Equal(t, "500", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, `{"message":"error from album-service Malformed JSON returned"}`, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "201", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"error from album-service Malformed JSON returned"}`, returnedBody)
}

func Test_postAlbums_Failure_Bad_Request(t *testing.T) {
	testRecorder, spanRecorder, router := setupTestRouter()
	DefaultClient = &StubClient{}

	requestBody := `{"Wu-Tang":"is for the children"}`
	requestBodyReader := io.NopCloser(bytes.NewReader([]byte(requestBody)))

	//inject in failure message to respond with that we could not get to the album-service
	responseBody := `{"artist":"Black Sabbath","id":10,"price":66.6,"title":"The Ozzman Cometh"}`
	body := io.NopCloser(bytes.NewReader([]byte(responseBody)))

	//inject a failure message from the server and return a json blob that represents an album
	StubResponseFunc = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       body,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/albums", requestBodyReader)
	router.ServeHTTP(testRecorder, req)
	byteArr, _ := io.ReadAll(testRecorder.Body)
	returnedBody := string(byteArr)

	assert.Equal(t, http.StatusBadRequest, testRecorder.Code)

	finishedSpans := spanRecorder.Ended()
	assert.Len(t, finishedSpans, 1)

	assert.Equal(t, codes.Error, finishedSpans[0].Status().Code)
	assert.Equal(t, "album-service returned error postAlbum", finishedSpans[0].Status().Description)

	assert.Equal(t, 1, len(finishedSpans[0].Events()))
	assert.Equal(t, "album-service returned error postAlbum", finishedSpans[0].Events()[0].Name)

	attributeMap := makeKeyMap(finishedSpans[0].Attributes())
	assert.Equal(t, requestBody, attributeMap["proxy-service.request.body"].Emit())

	assert.Equal(t, "400", attributeMap["proxy-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["proxy-service.response.body"].Emit())

	assert.Equal(t, "400", attributeMap["album-service.response.code"].Emit())
	assert.Equal(t, responseBody, attributeMap["album-service.response.body"].Emit())

	assert.Equal(t, `{"errors":null,"message":"album-service returned error postAlbum"}`, returnedBody)
}

func Test_getSwagger(t *testing.T) {
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
