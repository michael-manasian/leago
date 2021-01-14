package leago

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/throttled/throttled/v2"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

var (
	startingAddress = "https://%s.api.riotgames.com%s"
	tokenKey        = "X-Riot-Token"
	limitQuantity   = 1
)

type client struct {
	// A client that implements the functionality of
	// interacting with League of Legends game data.

	// Riot API access token, which can only be obtained from their portal.
	// To learn more: https://developer.riotgames.com/
	AccessToken string

	// An HTTP client through which requests can be made.
	HTTPClient *http.Client

	// A field that provides a limit on the number of requests
	// in a certain period of time, which avoids the 429 error.
	Limiter *throttled.GCRARateLimiter
}

func NewClient(accessToken string, httpClient *http.Client, limiter *throttled.GCRARateLimiter) *client {
	return &client{
		AccessToken: accessToken,
		HTTPClient:  httpClient,
		Limiter:     limiter,
	}
}

func (c *client) doRequest(ctx context.Context, region, api string, structure interface{}) error {
	request, err := c.makeRequest(ctx, region, api)
	if err != nil {
		return err
	}
	if err := c.applyLimit(api); err != nil {
		return err
	}

	body, err := c.getResponseBody(request)
	if err != nil {
		return err
	}
	return c.unloadBody(body, structure)
}

func (c *client) getResponseBody(request *http.Request) (io.ReadCloser, error) {
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		err, found := responseErrors[response.StatusCode]
		if !found {
			err = somethingWentWrong
		}
		return nil, err
	}
	return response.Body, nil
}

func (c *client) unloadBody(body io.ReadCloser, structure interface{}) error {
	readyBody, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}
	if err := body.Close(); err != nil {
		return err
	}
	return json.Unmarshal(readyBody, structure)
}

func (c *client) makeRequest(ctx context.Context, region, api string) (*http.Request, error) {
	if !availabilityCheck(region) {
		return nil, regionNotSupported
	}
	fullRequestPath := fmt.Sprintf(startingAddress, region, api)
	request, err := http.NewRequestWithContext(ctx, "GET", fullRequestPath, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set(tokenKey, c.AccessToken)
	return request, nil
}

func (c *client) applyLimit(api string) error {
	for {
		limited, ctx, err := c.Limiter.RateLimit(api, limitQuantity)
		if err != nil {
			return err
		}
		if !limited {
			return nil
		}
		time.Sleep(ctx.RetryAfter)
	}
}
