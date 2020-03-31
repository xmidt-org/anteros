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
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Comcast/webpa-common/logging"
	"github.com/Comcast/webpa-common/xmetrics"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/viper"
)

// newClient returns an http.Client to make send requests
func newClient(timeout time.Duration, scheme string) (client http.Client) {
	tr := &http.Transport{
		TLSHandshakeTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		DisableKeepAlives: true,
	}

	if strings.HasPrefix(scheme, "https") {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	client = http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: tr,
		Timeout:   timeout,
	}

	return
}

// returns a requests scheme
func getRequestScheme(req *http.Request) (scheme string) {
	scheme = "http"
	if req.TLS != nil {
		scheme = "https"
	}

	return
}

// newRedirectRequest returns a copy of the request but with a new host value.
func (h *primaryHandler) newRedirectRequest(req *http.Request, hostValue string) (reqCopy *http.Request, err error) {
	u := *req.URL
	u.Scheme = getRequestScheme(req)
	u.Host = hostValue

	// copy request body
	if req.Body == nil {
		reqCopy, err = http.NewRequest(req.Method, u.String(), nil)
	} else {
		defer req.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(req.Body)
		if err != nil {
			level.Error(h.logger).Log(logging.MessageKey(), "Error reading request body", "Error", err.Error())
			return
		}
		body := ioutil.NopCloser(bytes.NewBuffer(b))
		req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		reqCopy, err = http.NewRequest(req.Method, u.String(), body)
	}
	if err != nil {
		level.Error(h.logger).Log(logging.MessageKey(), "Error creating request copy.", "Error", err.Error())
		return
	}
	reqCopy.Header = req.Header

	return
}

// redirect sends an http.Request to the host supplied and sends the response back on a channel.
func (h *primaryHandler) redirect(respResultChan chan *respResult, req *http.Request, rr *respResult) {
	rr.response, rr.err = h.client.Do(req)
	if rr.err != nil {
		level.Error(h.logger).Log(logging.MessageKey(), "Redirect host response error.", "Error", rr.err.Error())
		respResultChan <- rr
		return
	}

	respResultChan <- rr
}

// evaluateXMiDTResponse determines if a XMiDT response is valid
func (h *primaryHandler) evaluateXMiDTResponse(hostResp *http.Response) (ok bool) {
	// check for nil response
	if hostResp == nil {
		level.Debug(h.logger).Log(logging.MessageKey(), "XMiDT host response was nil")
		return
	}

	level.Debug(h.logger).Log(logging.MessageKey(), "XMiDT Response", "StatusCode", hostResp.StatusCode)

	// these responses are determined to be bad from XMiDT
	if hostResp.StatusCode == 400 ||
		hostResp.StatusCode == 404 ||
		hostResp.StatusCode == 500 ||
		hostResp.StatusCode == 503 ||
		hostResp.StatusCode == 504 {
		return
	}

	ok = true
	return
}

// collector waits for response results from both hosts or until timeout is reached.
func (h *primaryHandler) collector(hostRespChan chan *respResult, resultChan chan *hostResults) {
	results := new(hostResults)
	ticker := time.NewTicker(h.timeout)

	// collect responses
	var stopper bool
	for !stopper && (results.xmidt == nil || results.webpa == nil) {
		select {
		case <-ticker.C:
			level.Error(h.logger).Log(logging.MessageKey(), "Response Timeout")
			stopper = true
		case rr := <-hostRespChan:
			if rr.hostName == "xmidt" {
				results.xmidt = rr
				h.metrics.ResponseReceivedXMiDT.Add(1.0)
			}
			if rr.hostName == "webpa" {
				results.webpa = rr
				h.metrics.ResponseReceivedWebPA.Add(1.0)
			}
		}
	}

	resultChan <- results
}

// parseTimeout converts the responseTimeout configuration value into a time.Duration value
func parseTimeout(logger log.Logger, v *viper.Viper) (timeout time.Duration, err error) {
	// parse timeout configuration value
	timeout, err = time.ParseDuration(v.GetString("responseTimeout"))
	if err != nil {
		level.Error(logger).Log(logging.MessageKey(), "Error parsing response timeout as duration", "Error", err.Error())
	}

	return
}

type hostResults struct {
	xmidt *respResult
	webpa *respResult
}

type respResult struct {
	response  *http.Response
	hostName  string
	hostValue string
	err       error
}

type primaryHandler struct {
	client  http.Client
	metrics Metrics
	hosts   map[string]string
	logger  log.Logger
	timeout time.Duration
	v       *viper.Viper
}

// setClient sets the http.Client used for sending requests
func (h *primaryHandler) setClient(client http.Client) {
	h.client = client
}

func (h *primaryHandler) setHosts(hosts map[string]string) {
	h.hosts = hosts
}

func (h *primaryHandler) setMetrics(registry xmetrics.Registry) {
	h.metrics = AddMetrics(registry)
}

// setLogger sets a logger for the primary logger
func (h *primaryHandler) setLogger(logger log.Logger) {
	h.logger = logger
}

// setTimeout sets a time.Duration value
func (h *primaryHandler) setTimeout(timeout time.Duration) {
	h.timeout = timeout
}

// setViper stores the viper configuration
func (h *primaryHandler) setViper(v *viper.Viper) {
	h.v = v
}

// NewPrimaryHandler returns a new primaryHanndler with app
func NewPrimaryHandler(logger log.Logger, registry xmetrics.Registry, v *viper.Viper) (http.Handler, error) {
	h := &primaryHandler{}
	h.setLogger(logger)
	h.setMetrics(registry)
	h.setViper(v)
	h.setHosts(v.GetStringMapString("hostRedirects"))

	// get timeout value from configuration
	timeout, err := parseTimeout(logger, v)
	if err == nil {
		h.setTimeout(timeout)
	}

	// create client to send requests
	h.setClient(newClient(timeout, "https"))

	return h, err
}

func (h *primaryHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	var d time.Duration
	if h.timeout == d {
		level.Error(h.logger).Log(logging.MessageKey(), "handler timeout is nil")
		http.Error(resp, "internal server error", http.StatusInternalServerError)
	}

	// send requests to redirect hosts
	hostRespChan := make(chan *respResult, 10)
	resultChan := make(chan *hostResults, 1)
	go h.collector(hostRespChan, resultChan)
	for k, v := range h.hosts {
		rr := &respResult{hostName: k, hostValue: v}
		reqCopy, err := h.newRedirectRequest(req, rr.hostValue)
		if err != nil {
			rr.err = err
			hostRespChan <- rr
		} else {
			go h.redirect(hostRespChan, reqCopy, rr)
		}
	}

	// wait for responses
	results := <-resultChan
	close(hostRespChan)
	close(resultChan)

	// evaluate responses
	finalResponse := results.xmidt
	if h.evaluateXMiDTResponse(finalResponse.response) {
		h.metrics.ResponseUsedXMiDT.Add(1.0)
	} else {
		level.Debug(h.logger).Log(logging.MessageKey(), "XMiDT response was determined to be unacceptable.  Using WebPA request.")
		finalResponse = results.webpa
		h.metrics.ResponseUsedWebPA.Add(1.0)
	}
	if finalResponse.err != nil {
		http.Error(resp, finalResponse.err.Error(), http.StatusInternalServerError)
	}

	level.Debug(h.logger).Log(logging.MessageKey(), "Final Response", "host", finalResponse.hostName, "StatusCode", finalResponse.response.StatusCode)

	// create response to return
	defer finalResponse.response.Body.Close()
	b, err := ioutil.ReadAll(finalResponse.response.Body)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
	}

	// copy over response header values to ResponseWriter
	for key, values := range finalResponse.response.Header {
		for i, value := range values {
			if i == 0 {
				resp.Header().Set(key, value)
			} else {
				resp.Header().Add(key, value)
			}
		}
	}

	resp.WriteHeader(finalResponse.response.StatusCode)

	resp.Write(b)
}
