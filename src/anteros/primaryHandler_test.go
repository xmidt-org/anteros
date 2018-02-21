/**
 * Copyright 2017 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Comcast/webpa-common/xmetrics"
	"github.com/go-kit/kit/log"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// testMakeTestServer returns a server for test.
// remember to close the server when done.
func testMakeTestServer(code int) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(code)
				rw.Write([]byte("test server response message"))
			},
		),
	)
}

// testCheckError is a handy function to assert there is an error
func testCheckError(assert *assert.Assertions, err error) {
	if err != nil {
		assert.Error(err)
	}
}

// testMakeLogger returns a logger for tests
func testMakeLogger() log.Logger {
	return log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
}

// testMakeViper returns a viper instance for use with tests
func testMakeViper() *viper.Viper {
	v := viper.New()
	v.SetDefault("hostRedirects.webpa", "localhost:6000")
	v.SetDefault("hostRedirects.xmidt", "localhost:6001")
	v.SetDefault("responseTimeout", "60s")

	return v
}

// testMakePrimaryHandler returns a primaryHandler for tests
func testMakePrimaryHandler(assert *assert.Assertions) *primaryHandler {
	v := testMakeViper()
	logger := testMakeLogger()
	timeout, err := parseTimeout(logger, v)
	testCheckError(assert, err)
	registry, err := xmetrics.NewRegistry(nil)
	testCheckError(assert, err)
	
	p := &primaryHandler{
		client:  newClient(timeout, "https"),
		hosts:   v.GetStringMapString("hostRedirects"),
		logger:  logger,
		metrics: AddMetrics(registry),
		timeout: timeout,
		v:       v,
	}

	return p
}

func TestNewClient(t *testing.T) {
	assert := assert.New(t)

	timeout := time.Second * 60
	client := newClient(timeout, "https")

	assert.NotNil(client)
	assert.IsType(http.Client{}, client)
}

func TestGetRequestScheme(t *testing.T) {
	assert := assert.New(t)

	schemes := []string{"http", "https"}
	for _, scheme := range schemes {
		u := url.URL{
			Scheme: scheme,
			Host:   "localhost:6000",
			Path:   "api/v1/test",
		}

		req, err := http.NewRequest("GET", u.String(), nil)
		testCheckError(assert, err)

		if scheme == "https" {
			req.TLS = &tls.ConnectionState{}
		}

		result := getRequestScheme(req)
		assert.Equal(scheme, result)
	}
}

func TestNewRedirectRequest(t *testing.T) {
	assert := assert.New(t)

	clientRequest, err := http.NewRequest("GET", "http://localhost:5000/foo/bar", nil)
	testCheckError(assert, err)
	clientRequest.Header.Set("X-Foo-Bar", "goomoo")

	p := testMakePrimaryHandler(assert)

	for _, value := range p.hosts {
		reqCopy, err := p.newRedirectRequest(clientRequest, value)
		testCheckError(assert, err)

		assert.Equal(value, reqCopy.URL.Host)
		assert.Equal(clientRequest.URL.Scheme, reqCopy.URL.Scheme)
		assert.Equal(clientRequest.Header, reqCopy.Header)
		assert.Equal(clientRequest.Body, reqCopy.Body)
	}
}

func TestRedirect(t *testing.T) {
	assert := assert.New(t)

	p := testMakePrimaryHandler(assert)

	for hostname := range p.hosts {
		server := testMakeTestServer(http.StatusOK)
		defer server.Close()
		serverURL, err := url.Parse(server.URL)
		testCheckError(assert, err)

		respChan := make(chan *respResult, 10)
		req, err := http.NewRequest("GET", serverURL.String(), nil)
		testCheckError(assert, err)
		rr := &respResult{
			hostName:  hostname,
			hostValue: serverURL.Host,
		}

		p := testMakePrimaryHandler(assert)
		go p.redirect(respChan, req, rr)
		rr = <-respChan
		close(respChan)

		assert.Equal(hostname, rr.hostName)
		assert.Equal(serverURL.Host, rr.hostValue)
		assert.NotNil(rr.response)
	}
}

func TestEvaluateXMiDTResponse(t *testing.T) {
	assert := assert.New(t)

	type record struct {
		code   int
		assert func(bool, ...interface{}) bool
	}

	records := []record{
		{200, assert.True},
		{400, assert.False},
		{404, assert.False},
		{500, assert.False},
		{503, assert.False},
		{504, assert.False},
	}

	p := testMakePrimaryHandler(assert)

	for _, record := range records {
		resp := &http.Response{
			StatusCode: record.code,
		}
		ok := p.evaluateXMiDTResponse(resp)
		record.assert(ok)
	}
}

func TestCollector(t *testing.T) {
	assert := assert.New(t)

	type record struct {
		timeout time.Duration
		wait    time.Duration
		assert  func(interface{}, ...interface{}) bool
	}

	records := []record{
		{time.Second * 60, time.Second * 0, assert.NotNil},
		{time.Second * 1, time.Second * 3, assert.Nil},
	}

	for _, record := range records {
		p := testMakePrimaryHandler(assert)
		p.setTimeout(record.timeout)

		respChan := make(chan *respResult, 10)
		resultChan := make(chan *hostResults, 1)
		go p.collector(respChan, resultChan)

		time.Sleep(record.wait)

		respChan <- &respResult{
			response:  &http.Response{StatusCode: 404},
			hostName:  "xmidt",
			hostValue: "localhost:6000",
			err:       nil,
		}
		respChan <- &respResult{
			response:  &http.Response{StatusCode: 200},
			hostName:  "webpa",
			hostValue: "localhost:6001",
			err:       nil,
		}

		results := <-resultChan
		close(respChan)
		close(resultChan)

		record.assert(results.xmidt)
		record.assert(results.webpa)
	}
}

func TestParseTimeout(t *testing.T) {
	assert := assert.New(t)

	i := 60
	timeout := time.Second * time.Duration(i)
	logger := testMakeLogger()
	v := viper.New()
	v.SetDefault("responseTimeout", fmt.Sprintf("%ds", i))

	result, err := parseTimeout(logger, v)
	testCheckError(assert, err)

	assert.Equal(timeout, result)
}

func TestNewPrimaryHandler(t *testing.T) {
	assert := assert.New(t)

	p := testMakePrimaryHandler(assert)

	assert.NotNil(p)
}

func TestServeHTTP(t *testing.T) {
	assert := assert.New(t)

	codes := []map[string]int{
		{"xmidt": http.StatusBadRequest, "webpa": http.StatusOK, "expected": http.StatusOK},          // should choose webpa
		{"xmidt": http.StatusNotFound, "webpa": http.StatusOK, "expected": http.StatusOK},            // should choose webpa
		{"xmidt": http.StatusInternalServerError, "webpa": http.StatusOK, "expected": http.StatusOK}, // should choose webpa
		{"xmidt": http.StatusServiceUnavailable, "webpa": http.StatusOK, "expected": http.StatusOK},  // should choose webpa
		{"xmidt": http.StatusGatewayTimeout, "webpa": http.StatusOK, "expected": http.StatusOK},      // should choose webpa

		{"xmidt": http.StatusBadRequest, "webpa": http.StatusNotFound, "expected": http.StatusNotFound},                               // should choose webpa
		{"xmidt": http.StatusNotFound, "webpa": http.StatusBadRequest, "expected": http.StatusBadRequest},                             // should choose webpa
		{"xmidt": http.StatusInternalServerError, "webpa": http.StatusGatewayTimeout, "expected": http.StatusGatewayTimeout},          // should choose webpa
		{"xmidt": http.StatusServiceUnavailable, "webpa": http.StatusInternalServerError, "expected": http.StatusInternalServerError}, // should choose webpa
		{"xmidt": http.StatusGatewayTimeout, "webpa": http.StatusServiceUnavailable, "expected": http.StatusServiceUnavailable},       // should choose webpa

		{"xmidt": http.StatusOK, "webpa": http.StatusNotFound, "expected": http.StatusOK}, // should choose xmidt
		{"xmidt": http.StatusOK, "webpa": http.StatusAccepted, "expected": http.StatusOK}, // should choose xmidt

		{"xmidt": http.StatusAccepted, "webpa": http.StatusOK, "expected": http.StatusAccepted},             // should choose xmidt
		{"xmidt": http.StatusForbidden, "webpa": http.StatusOK, "expected": http.StatusForbidden},           // should choose xmidt
		{"xmidt": http.StatusNotImplemented, "webpa": http.StatusOK, "expected": http.StatusNotImplemented}, // should choose xmidt
	}

	for _, code := range codes {
		// xmidt server
		serverXMiDT := testMakeTestServer(code["xmidt"])
		defer serverXMiDT.Close()
		serverXMiDTURL, err := url.Parse(serverXMiDT.URL)
		testCheckError(assert, err)

		// webpa server
		serverWebPA := testMakeTestServer(code["webpa"])
		defer serverWebPA.Close()
		serverWebPAURL, err := url.Parse(serverWebPA.URL)
		testCheckError(assert, err)

		// anteros server
		p := testMakePrimaryHandler(assert)
		hosts := map[string]string{"xmidt": serverXMiDTURL.Host, "webpa": serverWebPAURL.Host}
		p.setHosts(hosts)
		server := httptest.NewServer(p)
		defer server.Close()
		serverURL, err := url.Parse(server.URL)
		testCheckError(assert, err)

		resp, err := http.Get(serverURL.String())
		testCheckError(assert, err)

		assert.NotNil(resp)
		assert.Equal(resp.StatusCode, code["expected"])
	}
}
