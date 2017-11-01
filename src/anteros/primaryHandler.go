package main

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
	
	"github.com/Comcast/webpa-common/logging"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)


// newClient returns an http.Client to make send requests
func newClient(logger log.Logger, v *viper.Viper, scheme string) (client http.Client, err error) {
	timeout, err := time.ParseDuration( v.GetString("responseTimeout") )
	if err != nil {
		level.Error(logger).Log( logging.MessageKey(), "Error parsing response timeout as duration", "Error", err )
		return
	}
	
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

// redirect sends an http.Request to the host supplied.
func redirect(logger log.Logger, client http.Client, respResultChan chan *respResult, req *http.Request, rr *respResult) {
	u := *req.URL
	u.Scheme = req.URL.Scheme
	u.Host = rr.hostValue
	reqCopy, err := http.NewRequest(req.Method, u.String(), req.Body)  // todo: this should be better
	if err != nil {
		level.Error(logger).Log( logging.MessageKey(), "Error creating reqeust copy.", "Request", reqCopy, "Error", err )
		return
	}
	reqCopy.Header = req.Header
	
	rr.response, rr.err = client.Do(reqCopy)
	if rr.err != nil {
		level.Error(logger).Log( logging.MessageKey(), "Redirect host response error.", "Request", reqCopy, "Error", rr.err )
		return
	}
	
	respResultChan <- rr
}

// responseEvaluation determines if a XMiDT response is valid
func responseEvaluation(logger log.Logger, hostResp *http.Response) (ok bool) {
	// check for nil response
	if hostResp == nil {
		level.Debug(logger).Log( logging.MessageKey(), "XMiDT host response was nil" )
		return
	}
	
	level.Debug(logger).Log( logging.MessageKey(), "XMiDT Response", "StatusCode", hostResp.StatusCode)
	
	if hostResp.StatusCode == 400 ||
	   hostResp.StatusCode == 404 ||
	   hostResp.StatusCode == 500 ||
	   hostResp.StatusCode == 503 ||
	   hostResp.StatusCode == 504 {
		return
	}
	
	return
}

type respResult struct {
	response  *http.Response
	hostName  string
	hostValue string
	err       error
}

func NewPrimaryHandler(logger log.Logger, v *viper.Viper) (http.Handler, error) {
	router := mux.NewRouter()
	
	h := func(resp http.ResponseWriter, req *http.Request) {
		// create client to send requests
		client, err := newClient(logger, v, req.URL.Scheme)
		if err != nil {
			http.Error( resp, err.Error(), http.StatusInternalServerError )
		}
		
		// send requests to redirect hosts
		hosts := v.GetStringMapString("hostRedirects")
		hostRespChan := make(chan *respResult, 1)
		for k, v := range hosts {
			go redirect(logger, client, hostRespChan, req, &respResult{hostName: k, hostValue: v})
		}
		
		// collect responses
		var respXMiDT *respResult
		var respWebPA *respResult
		for respXMiDT == nil || respWebPA == nil {
			select {
			case rr := <- hostRespChan:
				if rr.hostName == "xmidt" { 
					respXMiDT = rr
				}
				if rr.hostName == "webpa" {
					respWebPA = rr
				}
			}
		}
		close(hostRespChan)
		
		// evaluate responses
		finalResponse := respXMiDT
		if !responseEvaluation(logger, respXMiDT.response) {
			level.Debug(logger).Log( logging.MessageKey(), "XMiDT response was determined to be unacceptable.  Using WebPA request." )
			finalResponse = respWebPA
		}
		if finalResponse.err != nil {
			http.Error( resp, finalResponse.err.Error(), http.StatusInternalServerError )
		}
		
		level.Debug(logger).Log( logging.MessageKey(), "Final Response", "host", finalResponse.hostName, "StatusCode", finalResponse.response.StatusCode )
		
		// create response to return
		b, err := ioutil.ReadAll(finalResponse.response.Body)
		if err != nil {
			http.Error( resp, err.Error(), http.StatusInternalServerError )
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
	router.PathPrefix("/").HandlerFunc(h)
	
	return router, nil
}
