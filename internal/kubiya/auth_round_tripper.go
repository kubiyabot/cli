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

func NewAuthRoundTripper() *AuthRoundTripper {
	tp := &AuthRoundTripper{
		Type:      "UserKey",
		Token:     os.Getenv("KUBIYA_API_KEY"),
		Transport: http.DefaultTransport,
	}

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
