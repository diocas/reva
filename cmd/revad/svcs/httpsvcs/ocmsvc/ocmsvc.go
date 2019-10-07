// Copyright 2018-2019 CERN
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// In applying this license, CERN does not waive the privileges and immunities
// granted to it by virtue of its status as an Intergovernmental Organization
// or submit itself to any jurisdiction.

package ocmsvc

import (
	"net/http"

	"github.com/cs3org/reva/cmd/revad/httpserver"
	"github.com/cs3org/reva/cmd/revad/svcs/httpsvcs"
	"github.com/mitchellh/mapstructure"
)

func init() {
	httpserver.Register("ocmsvc", New)
}

type config struct {
	Prefix string `mapstructure:"prefix"`
	Host   string `mapstructure:"public_host"`
	Webdav string `mapstructure:"webdav_endoint"`
}

type svc struct {
	prefix          string
	ProviderHandler *ProviderHandler
	SharesHandler   *SharesHandler
}

// New  returns a service with the OCM implementation
func New(m map[string]interface{}) (httpsvcs.Service, error) {
	conf := &config{}
	if err := mapstructure.Decode(m, conf); err != nil {
		return nil, err
	}

	if conf.Prefix == "" {
		conf.Prefix = "ocm"
	}

	if conf.Host == "" {
		conf.Host = "http://localhost"
	}

	service := &svc{
		prefix:          conf.Prefix,
		ProviderHandler: NewProviderHandler(conf.Host, conf.Prefix, conf.Webdav),
		SharesHandler:   new(SharesHandler),
	}

	return service, nil
}

func (s *svc) Close() error {
	return nil
}

func (s *svc) Prefix() string {
	return s.prefix
}

func (s *svc) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var head string
		head, r.URL.Path = httpsvcs.ShiftPath(r.URL.Path)
		switch head {
		case "ocm-provider":
			s.ProviderHandler.ServeHTTP(w, r)
		case "shares":
			s.SharesHandler.ServeHTTP(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}
