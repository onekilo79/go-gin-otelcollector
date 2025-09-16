
## Purpose

This is an example Go-gin application that demonstrates nested spans.

It does this by wrapping calls from the proxy-service to the album-service

This uses the opentelemetry instrumented http client [otelhttp](https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/instrumentation/net/http/otelhttp) 

The proxy-service has implemented some of the OpenTelemetry DefaultClient functions to allow a stub to be injected and return canned data. 

### Inspired by 

inspired by this for setting up gin & otel to test spans

[OpenTelemetry gin trace tests](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/instrumentation/github.com/gin-gonic/gin/otelgin/test/gintrace_test.go)

Further inspiration was taken from [ThegreatCodeAdventure Mocking Http Requests in GoLang](https://www.thegreatcodeadventure.com/mocking-http-requests-in-golang/)


## Testing with Stubs

The tests for the proxy-service use a stub service to send canned data back to the test code so we can test the proxy-service in isolation.

This allows the proxy service to be tested in isolation without needing to fire up both services. 

This is done by exposing the OpenTelemetry DefaulClient which does http requests and injecting the stub in to respond to test Http calls.

Not all tests need to override the DefaultClient behaviour so it is not done in the setup method.



## Prerequisites 
Cluster must have the following deployed
* Jaeger
* Opentelemetry-collector
* album-service


# Run 

## Docker-Compose

[Docker-Compose](../docs/Run-Docker-Compose-Install-Limited.md)

# K3d Run

[k3D install service ](../docs/K3D-run.md)
