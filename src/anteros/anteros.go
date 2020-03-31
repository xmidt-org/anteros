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
	"fmt"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/Comcast/webpa-common/concurrent"
	"github.com/Comcast/webpa-common/logging"
	"github.com/Comcast/webpa-common/server"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	applicationName = "anteros"
	release         = "Developer"
)

// anteros is the driver function for Anteros.  It performs everything main() would do,
// except for obtaining the command-line arguments (which are passed to it).
func anteros(arguments []string) int {
	//
	// Initialize the server environment: command-line flags, Viper, logging, and the WebPA instance
	//

	var (
		f = pflag.NewFlagSet(applicationName, pflag.ContinueOnError)
		v = viper.New()

		logger, metricsRegistry, webPA, err = server.Initialize(applicationName, arguments, f, v)
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialize Viper environment: %s\n", err)
		return 1
	}

	logger.Log(level.Key(), level.InfoValue(), "configurationFile", v.ConfigFileUsed())

	primaryHandler, err := NewPrimaryHandler(logger, metricsRegistry, v)
	if err != nil {
		logger.Log(level.Key(), level.ErrorValue(), logging.ErrorKey(), err, logging.MessageKey(), "unable to create primary handler")
		return 2
	}

	var (
		_, anterosServer = webPA.Prepare(logger, nil, metricsRegistry, primaryHandler)
		signals     = make(chan os.Signal, 1)
	)
	signal.Notify(signals, os.Interrupt, os.Kill)

	//
	// Execute the anterosServer, which runs all the servers, and wait for a signal
	//
	waitGroup, shutdown, err := concurrent.Execute(anterosServer)
	if err != nil {
		logger.Log(logging.MessageKey(), "Unable to start anteros", logging.ErrorKey(), err)
		return 4
	}

	signal.Notify(signals)
	s := server.SignalWait(logger, signals, os.Kill, os.Interrupt)
	logger.Log(logging.MessageKey(), "exiting due to signal", "signal", s)
	close(shutdown)
	waitGroup.Wait()

	return 0
}

func main() {
	os.Exit(anteros(os.Args))
}
