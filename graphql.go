package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/shurcooL/graphql/internal/jsonutil"
)

// Client is a GraphQL client.
type Client struct {
	url            string // GraphQL server URL.
	httpClient     *http.Client
	requestOptions []RequestOption
}

// NewClient creates a GraphQL client targeting the specified GraphQL server URL.
// If httpClient is nil, then http.DefaultClient is used.
func NewClient(url string, httpClient *http.Client, opts ...RequestOption) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		url:            url,
		httpClient:     httpClient,
		requestOptions: opts,
	}
}

// Query executes a single GraphQL query request,
// with a query derived from q, populating the response into it.
// q should be a pointer to struct that corresponds to the GraphQL schema.
func (c *Client) Query(ctx context.Context, q interface{}, variables map[string]interface{}, opts ...RequestOption) error {
	return c.do(ctx, queryOperation, q, variables, opts)
}

// Mutate executes a single GraphQL mutation request,
// with a mutation derived from m, populating the response into it.
// m should be a pointer to struct that corresponds to the GraphQL schema.
func (c *Client) Mutate(ctx context.Context, m interface{}, variables map[string]interface{}, opts ...RequestOption) error {
	return c.do(ctx, mutationOperation, m, variables, opts)
}

// do executes a single GraphQL operation.
func (c *Client) do(ctx context.Context, op operationType, v interface{}, variables map[string]interface{}, opts []RequestOption) error {
	var query string
	switch op {
	case queryOperation:
		query = constructQuery(v, variables)
	case mutationOperation:
		query = constructMutation(v, variables)
	}
	in := struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables,omitempty"`
	}{
		Query:     query,
		Variables: variables,
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	var allOpts []RequestOption
	allOpts = append(allOpts, c.requestOptions...)
	allOpts = append(allOpts, opts...)

	for _, opt := range allOpts {
		if err := opt(req); err != nil {
			return &OptionError{Err: err}
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("non-200 OK status code: %v body: %q", resp.Status, body)
	}
	var out struct {
		Data   *json.RawMessage
		Errors GraphQLErrors
	}
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		// TODO: Consider including response body in returned error, if deemed helpful.
		return err
	}
	if out.Data != nil {
		err := jsonutil.UnmarshalGraphQL(*out.Data, v)
		if err != nil {
			// TODO: Consider including response body in returned error, if deemed helpful.
			return err
		}
	}
	if len(out.Errors) > 0 {
		return out.Errors
	}
	return nil
}

// GraphQLErrors represents the "GraphQLErrors" array in a response from a GraphQL server.
// If returned via error interface, the slice is expected to contain at least 1 element.
//
// Specification: https://facebook.github.io/graphql/#sec-Errors.
// Actual implementation:
// https://github.com/spacelift-io/graphql-go/blob/4c5b960673418ee4577498869c8dfa2c66628458/GraphQLErrors/GraphQLErrors.go#L7
type GraphQLErrors []struct {
	Message   string
	Locations []struct {
		Line   int
		Column int
	}
	Path       []interface{}
	Extensions map[string]interface{}
}

// Error implements error interface.
func (e GraphQLErrors) Error() string {
	return e[0].Message
}

type operationType uint8

const (
	queryOperation operationType = iota
	mutationOperation
	//subscriptionOperation // Unused.
)
