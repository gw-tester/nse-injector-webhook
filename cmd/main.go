/*
Copyright 2021
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	arg "github.com/alexflint/go-arg"
	"github.com/gw-tester/nse-injector-webhook/internal/core/domain"
	router "github.com/gw-tester/nse-injector-webhook/internal/routers/webhookrouter"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type args struct {
	Log        logLevel `arg:"env:LOG_LEVEL" default:"info" help:"Defines the level of logging for this program."`
	Port       int      `default:"8443" help:"Webhook server port."`
	TLSCert    file     `help:"File containing the x509 Certificate for HTTPS."`
	TLSKey     file     `help:"File containing the x509 private key to the certificate."`
	SidecarCfg file     `help:"File containing the mutation configuration."`
}

type logLevel struct {
	Level log.Level
}

func (n *logLevel) UnmarshalText(b []byte) error {
	s := string(b)

	logLevel, err := log.ParseLevel(s)
	if err != nil {
		return errors.Wrap(err, "failed to parse the log level")
	}

	n.Level = logLevel

	return nil
}

//nolint:unparam
func (n *logLevel) MarshalText() ([]byte, error) {
	return []byte(log.InfoLevel.String()), nil
}

type file struct {
	File string
}

func (f *file) UnmarshalText(b []byte) error {
	s := string(b)

	if _, err := os.Stat(s); os.IsNotExist(err) {
		return errors.Wrapf(err, "%v file doesn't exist", s)
	}

	f.File = s

	return nil
}

func (args) Version() string {
	return "nse-injector 0.0.2"
}

func (args) Description() string {
	return "this program injects NSE sidecar into the pod description"
}

func main() {
	var args args

	arg.MustParse(&args)
	log.SetLevel(args.Log.Level)

	pair, err := tls.LoadX509KeyPair(args.TLSCert.File, args.TLSKey.File)
	if err != nil {
		log.Fatalf("Failed to load key pair: %v", err)
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%v", args.Port),
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{pair},
		},
	}

	config, err := domain.GetConfig(args.SidecarCfg.File)
	if err != nil {
		log.WithError(err).Fatal("Failed to load NSE configuration file")
	}

	router := router.New(server, config)
	if router == nil {
		log.Panic("Failed to initialize NSE webhook server")
	}
	defer router.Close()

	router.ListenAndServe()
}
