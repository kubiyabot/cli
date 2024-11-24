package kubiya

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) ListSecrets(ctx context.Context) ([]Secret, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/secrets", c.baseURL), nil)
	if err != nil {
		return nil, err
	}

	var secrets []Secret
	if err := c.do(req, &secrets); err != nil {
		return nil, err
	}

	return secrets, nil
}

func (c *Client) SetSecret(ctx context.Context, name, value, description string) error {
	payload := struct {
		Value       string `json:"value"`
		Description string `json:"description,omitempty"`
	}{
		Value:       value,
		Description: description,
	}

	req, err := c.newJSONRequest(ctx, "POST", fmt.Sprintf("%s/secrets/%s", c.baseURL, name), payload)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/secrets/%s", c.baseURL, name), nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}
