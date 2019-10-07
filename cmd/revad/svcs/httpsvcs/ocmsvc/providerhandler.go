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
	"encoding/json"
	"net/http"
	"net/url"
	"path"
)

type ProviderHandler struct {
	host   string
	prefix string
	webdav string
}

func NewProviderHandler(host, prefix, webdav string) *ProviderHandler {

	handler := &ProviderHandler{
		host:   host,
		prefix: prefix,
		webdav: webdav,
	}
	return handler
}

func (h *ProviderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	HostUrl, _ := url.Parse(h.host)
	HostUrl.Path = path.Join(HostUrl.Path, h.prefix)

	info := &Info{
		Enabled:    true,
		APIVersion: "1.0-proposal1",
		EndPoint:   HostUrl.String(),
		ShareTypes: []ShareTypes{ShareTypes{
			Name: "file",
			Protocols: ShareTypesProtocols{
				Webdav: h.webdav,
			},
		}},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(info.JSON())
}

type Info struct {
	Enabled    bool         `json:"enabled"`
	APIVersion string       `json:"apiVersion"`
	EndPoint   string       `json:"endPoint"`
	ShareTypes []ShareTypes `json:"shareTypes"`
}

type ShareTypes struct {
	Name      string              `json:"name"`
	Protocols ShareTypesProtocols `json:"protocols"`
}

type ShareTypesProtocols struct {
	Webdav string `json:"webdav"`
}

func (s *Info) JSON() []byte {
	b, _ := json.MarshalIndent(s, "", "   ")
	return b

}
