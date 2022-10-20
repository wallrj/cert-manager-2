/*
Copyright 2020 The cert-manager Authors.

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

package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	vcert "github.com/Venafi/vcert/v4"
	"github.com/Venafi/vcert/v4/pkg/certificate"
	"github.com/Venafi/vcert/v4/pkg/endpoint"
	"github.com/Venafi/vcert/v4/pkg/venafi/tpp"
	"github.com/go-logr/logr"
	corelisters "k8s.io/client-go/listers/core/v1"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/cert-manager/cert-manager/pkg/issuer/venafi/client/api"
	"github.com/cert-manager/cert-manager/pkg/metrics"
)

const (
	tppUsernameKey    = "username"
	tppPasswordKey    = "password"
	tppAccessTokenKey = "access-token"

	defaultAPIKeyKey = "api-key"
)

type VenafiClientBuilder func(namespace string, secretsLister corelisters.SecretLister,
	issuer cmapi.GenericIssuer, metrics *metrics.Metrics, logger logr.Logger) (Interface, error)

// Interface implements a Venafi client
type Interface interface {
	RequestCertificate(csrPEM []byte, duration time.Duration, customFields []api.CustomField) (string, error)
	RetrieveCertificate(pickupID string, csrPEM []byte, duration time.Duration, customFields []api.CustomField) ([]byte, error)
	ReadZoneConfiguration() (*endpoint.ZoneConfiguration, error)
}

// connector exposes a subset of the vcert Connector interface to make stubbing
// out its functionality during tests easier.
type connector interface {
	ReadZoneConfiguration() (config *endpoint.ZoneConfiguration, err error)
	RequestCertificate(req *certificate.Request) (requestID string, err error)
	RetrieveCertificate(req *certificate.Request) (certificates *certificate.PEMCollection, err error)
}

// Venafi is a implementation of vcert library to manager certificates from TPP or Venafi Cloud
type Venafi struct {
	vcertClient connector
}

// New constructs a Venafi client Interface. Errors may be network errors and
// should be considered for retrying.
func New(namespace string, secretsLister corelisters.SecretLister, issuer cmapi.GenericIssuer, metrics *metrics.Metrics, logger logr.Logger) (Interface, error) {
	vcertClient, err := clientForIssuer(issuer, secretsLister, namespace)
	if err != nil {
		return nil, fmt.Errorf("error creating Venafi client: %s", err.Error())
	}
	return &Venafi{
		vcertClient: newInstumentedConnector(vcertClient, metrics, logger),
	}, nil
}

// clientForIssuer will convert a cert-manager Venafi issuer into a vcert client
func clientForIssuer(iss cmapi.GenericIssuer, secretsLister corelisters.SecretLister, namespace string) (endpoint.Connector, error) {
	venCfg := iss.GetSpec().Venafi
	switch {
	case venCfg.TPP != nil:
		tppCfg := venCfg.TPP
		tppSecret, err := secretsLister.Secrets(namespace).Get(tppCfg.CredentialsRef.Name)
		if err != nil {
			return nil, err
		}

		username := string(tppSecret.Data[tppUsernameKey])
		password := string(tppSecret.Data[tppPasswordKey])
		accessToken := string(tppSecret.Data[tppAccessTokenKey])
		caBundle := string(tppCfg.CABundle)

		// We use vcert.NewClient rather than tpp.NewClient because
		// vcert.NewClient takes care of parsing caBundle, setting zone etc.
		// But we skip the authentication because if a username / password is
		// supplied, vcert will implicitly use deprecated api-key
		// authentication.
		cli, err := vcert.NewClient(&vcert.Config{
			ConnectorType: endpoint.ConnectorTypeTPP,
			BaseUrl:       tppCfg.URL,
			Zone:          venCfg.Zone,
			// always enable verbose logging for now
			LogVerbose:      true,
			ConnectionTrust: caBundle,
			Client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						Renegotiation: tls.RenegotiateOnceAsClient,
					},
				},
			},
		}, false) // false makes vcert skip the authentication
		if err != nil {
			return nil, err
		}

		// If a username and password are supplied, perform Oauth authentication.
		// Otherwise it is assumed that an access-token (bearer token) has been
		// supplied which can be used when interacting with and Oauth is unnecessary.
		if username != "" && password != "" {
			// The Oauth authentication functions are not available in the generic
			// endpoint.Connector only in the tpp implementation of that interface,
			// which is why we have to do a type cast here.
			tpp, ok := cli.(*tpp.Connector)
			if !ok {
				return nil, fmt.Errorf("Program error: vcert.NewClient returned an unexpected endpoint.Connector type: %T", cli)
			}
			// GetRefreshToken will
			res, err := tpp.GetRefreshToken(&endpoint.Authentication{
				User:     username,
				Password: password,
				ClientId: "",
				Scope:    "",
			})
			if err != nil {
				return nil, err
			}
			accessToken = res.Access_token
		}

		if err := cli.Authenticate(&endpoint.Authentication{
			AccessToken: accessToken,
		}); err != nil {
			return nil, err
		}

		return cli, nil
	case venCfg.Cloud != nil:
		cloud := venCfg.Cloud
		cloudSecret, err := secretsLister.Secrets(namespace).Get(cloud.APITokenSecretRef.Name)
		if err != nil {
			return nil, err
		}

		k := defaultAPIKeyKey
		if cloud.APITokenSecretRef.Key != "" {
			k = cloud.APITokenSecretRef.Key
		}
		apiKey := string(cloudSecret.Data[k])

		return vcert.NewClient(&vcert.Config{
			ConnectorType: endpoint.ConnectorTypeCloud,
			BaseUrl:       cloud.URL,
			Zone:          venCfg.Zone,
			// always enable verbose logging for now
			LogVerbose: true,
			Credentials: &endpoint.Authentication{
				APIKey: apiKey,
			},
		})
	}
	// API validation in webhook and in the ClusterIssuer and Issuer controller
	// Sync functions should make this unreachable in production.
	return nil, fmt.Errorf("neither Venafi Cloud or TPP configuration found")
}

func (v *Venafi) ReadZoneConfiguration() (*endpoint.ZoneConfiguration, error) {
	return v.vcertClient.ReadZoneConfiguration()
}
