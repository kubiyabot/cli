package kubiya

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type AuthRoundTripper struct {
	Type      string
	Token     string
	Transport http.RoundTripper
}

func NewAuthRoundTripper(apiKey string) *AuthRoundTripper {
	tp := &AuthRoundTripper{
		Type:      "UserKey",
		Token:     apiKey,
		Transport: http.DefaultTransport,
	}

	// If no API key was provided, try environment variable
	if tp.Token == "" {
		tp.Token = os.Getenv("KUBIYA_API_KEY")
	}

	// If still no API key, try config file
	if tp.Token == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return tp
		}
		filePath := filepath.Join(homeDir, ".kubiya.json")

		data, err := os.ReadFile(filePath)
		if err != nil {
			return tp
		}

		type TokenFile struct {
			Type  string `json:"type"`
			Token string `json:"token"`
		}
		var tokenFile TokenFile
		if err := json.Unmarshal(data, &tokenFile); err != nil {
			return tp
		}

		if tokenFile.Token != "" {
			tp.Type = tokenFile.Type
			tp.Token = tokenFile.Token
		}
	}

	return tp
}

func (a *AuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if a.Token != "" {
		t := "%s %s"
		req.Header.Set("Authorization", fmt.Sprintf(t, a.Type, a.Token))
	}

	if a.Transport == nil {
		a.Transport = http.DefaultTransport
	}

	return a.Transport.RoundTrip(req)
}
