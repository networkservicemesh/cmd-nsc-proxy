// Copyright (c) 2020 Doc.ai and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/stretchr/testify/suite"

	"github.com/networkservicemesh/sdk/pkg/tools/spire"

	"github.com/sirupsen/logrus"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	main "github.com/networkservicemesh/cmd-nsc-proxy"
	"github.com/networkservicemesh/sdk/pkg/networkservice/chains/client"
	"github.com/networkservicemesh/sdk/pkg/networkservice/chains/endpoint"
	"github.com/networkservicemesh/sdk/pkg/networkservice/common/authorize"
	"github.com/networkservicemesh/sdk/pkg/tools/grpcutils"
	"github.com/networkservicemesh/sdk/pkg/tools/spiffejwt"
)

func TokenGenerator(peerAuthInfo credentials.AuthInfo) (token string, expireTime time.Time, err error) {
	return "TestToken", time.Date(3000, 1, 1, 1, 1, 1, 1, time.UTC), nil
}

type ProxyNSCSuite struct {
	suite.Suite

	ctx        context.Context
	cancel     context.CancelFunc
	spireErrCh <-chan error
}

func (f *ProxyNSCSuite) SetupSuite() {
	logrus.SetFormatter(&nested.Formatter{})
	logrus.SetLevel(logrus.TraceLevel)
	f.ctx, f.cancel = context.WithCancel(context.Background())

	// Run spire
	executable, err := os.Executable()
	require.NoError(f.T(), err)

	reuseSpire := os.Getenv(workloadapi.SocketEnv) != ""
	if !reuseSpire {
		f.spireErrCh = spire.Start(
			spire.WithContext(f.ctx),
			spire.WithEntry("spiffe://example.org/proxy-nsc", "unix:path:/bin/nsmgr"),
			spire.WithEntry("spiffe://example.org/proxy-nsc.test", "unix:uid:0"),
			spire.WithEntry(fmt.Sprintf("spiffe://example.org/%s", filepath.Base(executable)),
				fmt.Sprintf("unix:path:%s", executable),
			),
		)
	}
}
func (f *ProxyNSCSuite) TearDownSuite() {
	f.cancel()
	if f.spireErrCh != nil {
		for {
			_, ok := <-f.spireErrCh
			if !ok {
				break
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestProxyNSCTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyNSCSuite))
}

func (f *ProxyNSCSuite) TestProxyClient() {
	t := f.T()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Get a X509Source
	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		logrus.Fatalf("error getting x509 source: %+v", err)
	}
	var svid *x509svid.SVID
	svid, err = source.GetX509SVID()
	if err != nil {
		logrus.Fatalf("error getting x509 svid: %+v", err)
	}
	logrus.Infof("SVID: %q", svid.ID)

	mgr := endpoint.NewServer(ctx, "test", authorize.NewServer(), spiffejwt.TokenGeneratorFunc(source, time.Second*1000))

	mgrURL := &url.URL{Scheme: "tcp", Host: ":0"}
	endpoint.Serve(ctx, mgrURL, mgr, grpc.Creds(credentials.NewTLS(tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny()))))

	cfg := &main.Config{
		Name:             "proxy-nsc",
		ConnectTo:        *mgrURL,
		ListenOn:         url.URL{Scheme: "tcp", Host: ":0"}, // Some random port we would like to connect
		MaxTokenLifetime: time.Hour,
	}
	proxyClient, nsmClient, e := main.NewNSMProxyClient(ctx, cfg)
	require.NoError(t, e)

	errChan := main.RunProxyClient(ctx, cfg, proxyClient, nsmClient)
	require.NotNil(t, errChan)

	logrus.Infof("Proxy is listening on: %v", cfg.ListenOn)
	// Obtain a connection
	var clientCC *grpc.ClientConn
	clientCC, err = grpc.DialContext(ctx,
		grpcutils.URLToTarget(&cfg.ListenOn),
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(
			grpc.WaitForReady(true)))

	require.NoError(t, err)

	defer func() { _ = clientCC.Close() }()

	nsClient := client.NewClient(ctx, "nsc", nil, TokenGenerator, clientCC)

	var conn *networkservice.Connection
	conn, err = nsClient.Request(ctx, &networkservice.NetworkServiceRequest{})

	require.NoError(t, err)
	require.NotNil(t, conn)

	require.Equal(t, 2, len(conn.Path.PathSegments))

	require.NotEqual(t, "TestToken", conn.Path.PathSegments[0].Token)
	require.NotEqual(t, "TestToken", conn.Path.PathSegments[1].Token)
}
