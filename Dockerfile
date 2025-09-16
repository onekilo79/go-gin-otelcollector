# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# syntax = docker/dockerfile:1-experimental
FROM golang:1.20 as build
ARG GIT_HASH
WORKDIR /app/
COPY . .
RUN go mod download
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -ldflags "-X main.version=0.1 -X main.gitHash=${GIT_HASH}" -v -o album-service-bin main.go
FROM alpine:3.17.3
COPY --from=build /app/album-service-bin /app/album-service-bin
CMD ["/app/album-service-bin"]