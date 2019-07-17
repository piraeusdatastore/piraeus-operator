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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/moul/http2curl"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Client is a struct representing a LINSTOR REST client.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	logCfg     *LogCfg
	lim        *rate.Limiter
	log        *logrus.Entry

	Nodes               *NodeService
	ResourceDefinitions *ResourceDefinitionService
	Resources           *ResourceService
	Encryption          *EncryptionService
}

// LogCfg is a struct containing the client's logging configuration
type LogCfg struct {
	Out       io.Writer
	Formatter logrus.Formatter
	Level     string
}

// const errors as in https://dave.cheney.net/2016/04/07/constant-errors
type clientError string

func (e clientError) Error() string { return string(e) }

// NotFoundError is the error type returned in case of a 404 error. This is required to test for this kind of error.
const NotFoundError = clientError("404 Not Found")

// NewClient takes an arbitrary number of options and returns a Client or an error.
func NewClient(options ...func(*Client) error) (*Client, error) {
	httpClient := http.DefaultClient

	hostPort := "localhost:3370"
	controllers := os.Getenv("LS_CONTROLLERS")
	// we could ping them, for now use the first if possible
	if controllers != "" {
		hostPort = strings.Split(controllers, ",")[0]

		lsPrefix := "linstor://"
		if strings.HasPrefix(hostPort, lsPrefix) {
			hostPort = strings.TrimPrefix(hostPort, lsPrefix)
		}
	}

	if !strings.Contains(hostPort, ":") {
		hostPort += ":3370"
	}

	u := hostPort
	if !strings.HasPrefix(hostPort, "http://") {
		u = "http://" + hostPort
	}

	baseURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	c := &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		lim:        rate.NewLimiter(rate.Inf, 0),
		log:        logrus.NewEntry(logrus.New()),
	}
	l := &LogCfg{
		Level: logrus.WarnLevel.String(),
	}
	if err := Log(l)(c); err != nil {
		return nil, err
	}

	c.Nodes = &NodeService{client: c}
	c.ResourceDefinitions = &ResourceDefinitionService{client: c}
	c.Resources = &ResourceService{client: c}
	c.Encryption = &EncryptionService{client: c}

	for _, opt := range options {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Options for the Client
// For example:
// u, _ := url.Parse("http://somehost:3370")
// c, _ := linstor.NewClient(linstor.BaseURL(u))

// BaseURL is a client's option to set the baseURL of the REST client.
func BaseURL(URL *url.URL) func(*Client) error {
	return func(c *Client) error {
		c.baseURL = URL
		return nil
	}
}

// HTTPClient is a client's option to set a specific http.Client.
func HTTPClient(httpClient *http.Client) func(*Client) error {
	return func(c *Client) error {
		c.httpClient = httpClient
		return nil
	}
}

// Log is a client's option to set a LogCfg.
func Log(logCfg *LogCfg) func(*Client) error {
	return func(c *Client) error {
		c.logCfg = logCfg
		level, err := logrus.ParseLevel(c.logCfg.Level)
		if err != nil {
			return err
		}
		c.log.Logger.SetLevel(level)
		if c.logCfg.Out == nil {
			c.logCfg.Out = os.Stderr
		}
		c.log.Logger.SetOutput(c.logCfg.Out)
		if c.logCfg.Formatter != nil {
			c.log.Logger.SetFormatter(c.logCfg.Formatter)
		}
		return nil
	}
}

// Limit is the client's option to set number of requests per second and
// max number of bursts.
func Limit(r rate.Limit, b int) func(*Client) error {
	return func(c *Client) error {
		if b == 0 && r != rate.Inf {
			return fmt.Errorf("invalid rate limit, burst must not be zero for non-unlimted rates")
		}
		c.lim = rate.NewLimiter(r, b)
		return nil
	}
}

func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.baseURL.ResolveReference(rel)

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
		c.log.Debug(body)
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	// req.Header.Set("User-Agent", c.UserAgent)

	return req, nil
}

func (c *Client) curlify(req *http.Request) (string, error) {
	cc, err := http2curl.GetCurlCommand(req)
	if err != nil {
		return "", err
	}
	return cc.String(), nil
}

func (c *Client) logCurlify(req *http.Request, lvl logrus.Level) {
	// allow it to be called unconditionally; make it a noop if level does not match
	if !c.log.Logger.IsLevelEnabled(lvl) {
		return
	}

	if curl, err := c.curlify(req); err != nil {
		c.log.Println(err)
	} else {
		c.log.Println(curl)
	}
}

func (c *Client) do(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	if err := c.lim.Wait(ctx); err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	c.logCurlify(req, logrus.DebugLevel)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		c.log.Debugf("Status code not within 200 to 400, but %d (%s)\n",
			resp.StatusCode, http.StatusText(resp.StatusCode))
		if resp.StatusCode == 404 {
			return nil, NotFoundError
		}

		var rets []ApiCallRc
		if err = json.NewDecoder(resp.Body).Decode(&rets); err != nil {
			return nil, err
		}

		var finalErr string
		for i, e := range rets {
			finalErr += strings.TrimSpace(e.String())
			if i < len(rets)-1 {
				finalErr += " next error: "
			}
		}
		return nil, errors.New(finalErr)
	}

	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
	}
	return resp, err
}

// Higer Leve Abstractions

func (c *Client) doGET(ctx context.Context, url string, ret interface{}, opts ...*ListOpts) (*http.Response, error) {

	u, err := addOptions(url, genOptions(opts...))
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, req, ret)
}

func (c *Client) doPOST(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	req, err := c.newRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	return c.do(ctx, req, nil)
}

func (c *Client) doPUT(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	req, err := c.newRequest("PUT", url, body)
	if err != nil {
		return nil, err
	}

	return c.do(ctx, req, nil)
}

func (c *Client) doPATCH(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	req, err := c.newRequest("PATCH", url, body)
	if err != nil {
		return nil, err
	}

	return c.do(ctx, req, nil)
}

func (c *Client) doDELETE(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	req, err := c.newRequest("DELETE", url, body)
	if err != nil {
		return nil, err
	}

	return c.do(ctx, req, nil)
}

// ApiCallRc represents the struct returned by LINSTOR, when accessing its REST API.
type ApiCallRc struct {
	// A masked error number
	RetCode int64  `json:"ret_code"`
	Message string `json:"message"`
	// Cause of the error
	Cause string `json:"cause,omitempty"`
	// Details to the error message
	Details string `json:"details,omitempty"`
	// Possible correction options
	Correction string `json:"correction,omitempty"`
	// List of error report ids related to this api call return code.
	ErrorReportIds []string `json:"error_report_ids,omitempty"`
	// Map of objection that have been involved by the operation.
	ObjRefs map[string]string `json:"obj_refs,omitempty"`
}

func (rc *ApiCallRc) String() string {
	s := fmt.Sprintf("Message: '%s'", rc.Message)
	if rc.Cause != "" {
		s += fmt.Sprintf("; Cause: '%s'", rc.Cause)
	}
	if rc.Details != "" {
		s += fmt.Sprintf("; Details: '%s'", rc.Details)
	}
	if rc.Correction != "" {
		s += fmt.Sprintf("; Correction: '%s'", rc.Correction)
	}
	if len(rc.ErrorReportIds) > 0 {
		s += fmt.Sprintf("; Reports: '[%s]'", strings.Join(rc.ErrorReportIds, ","))
	}

	return s
}

// DeleteProps is a slice of properties to delete.
type DeleteProps []string

// OverrideProps is a map of properties to modify (key/value pairs)
type OverrideProps map[string]string

// Namespaces to delete
type DeleteNamespaces []string

// GenericPropsModify is a struct combining DeleteProps and OverrideProps
type GenericPropsModify struct {
	DeleteProps      DeleteProps      `json:"delete_props,omitempty"`
	OverrideProps    OverrideProps    `json:"override_props,omitempty"`
	DeleteNamespaces DeleteNamespaces `json:"delete_namespaces,omitempty"`
}
