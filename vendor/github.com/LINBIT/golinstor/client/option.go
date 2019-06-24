/*
* A REST client to interact with LINSTOR's REST API
* Copyright © 2019 LINBIT HA-Solutions GmbH
* Author: Roland Kammerer <roland.kammerer@linbit.com>
*
* This program is free software; you can redistribute it and/or modify
* it under the terms of the GNU General Public License as published by
* the Free Software Foundation; either version 2 of the License, or
* (at your option) any later version.
*
* This program is distributed in the hope that it will be useful,
* but WITHOUT ANY WARRANTY; without even the implied warranty of
* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
* GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License
* along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"net/url"
	"reflect"

	"github.com/google/go-querystring/query"
)

// ListOpts is a struct primarily used to define parameters used for pagination. It is also used for filtering (e.g., the /view/ calls)
type ListOpts struct {
	Page    int `url:"offset"`
	PerPage int `url:"limit"`

	StoragePool []string `url:"storage_pools"`
	Resource    []string `url:"resources"`
	Node        []string `url:"nodes"`
}

func genOptions(opts ...*ListOpts) *ListOpts {
	if opts == nil || len(opts) == 0 {
		return nil
	}

	return opts[0]
}

func addOptions(s string, opt interface{}) (string, error) {
	v := reflect.ValueOf(opt)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	vs, err := query.Values(opt)
	if err != nil {
		return s, err
	}

	u.RawQuery = vs.Encode()
	return u.String(), nil
}
