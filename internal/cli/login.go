package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
)

const (
	auth0Domain      = "kubiya.us.auth0.com"
	auth0ClientId    = "SxpP9OU7VSvvPivHFQY5Get3uC1Bx4Jf"
	auth0Scopes      = "openid profile email"
	auth0Audience    = "https://kubiya.ai/api"
	auth0RedirectURI = "http://127.0.0.1:9000/callback"
	callbackTimeout  = 5 * time.Minute // Timeout for waiting for callback
)

// DeviceCodeResponse represents the response from the device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the successful token response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// TokenErrorResponse represents error responses during polling.
type TokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

type UserInfo struct {
	Sub           string                 `json:"sub"`
	Name          string                 `json:"name,omitempty"`
	GivenName     string                 `json:"given_name,omitempty"`
	FamilyName    string                 `json:"family_name,omitempty"`
	Email         string                 `json:"email,omitempty"`
	EmailVerified bool                   `json:"email_verified,omitempty"`
	Picture       string                 `json:"picture,omitempty"`
	OrgName       string                 `json:"org_name,omitempty"`
	CustomClaims  map[string]interface{} `json:"-"` // For custom metadata (e.g., https://myapp.kubiya.ai/subscription_tier)
}

// newLoginCommand creates a new login command with the provided config.
func newLoginCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate using Auth0 device flow",
		Long: `Authenticate with Auth0 using the device flow for SPA applications.
This command initiates the device flow, opens the verification URL (optional),
and polls for the access token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			browser, _ := cmd.Flags().GetBool("open-browser")
			return runAuthCodeFlow(cfg, browser)
		},
	}

	// Define flags for the login command
	cmd.Flags().Bool("open-browser", true, "Automatically open browser for verification")

	return cmd
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}

// generateCodeVerifier creates a random code verifier for PKCE.
func generateCodeVerifier() (string, error) {
	const length = 43 // Must be 43-128 characters
	data := make([]byte, length)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// generateCodeChallenge computes the SHA-256 code challenge from the verifier.
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// runAuthCodeFlow executes the Auth0 Authorization Code Flow with PKCE.
func runAuthCodeFlow(cfg *config.Config, browser bool) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Step 1: Generate PKCE code verifier and challenge
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return fmt.Errorf("generating code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Step 2: Construct authorization URL
	authURL := fmt.Sprintf("https://%s/authorize", auth0Domain)
	params := url.Values{}
	params.Set("client_id", auth0ClientId)
	params.Set("response_type", "code")
	params.Set("redirect_uri", auth0RedirectURI)
	params.Set("scope", auth0Scopes)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("audience", auth0Audience)
	authorizeURL := authURL + "?" + params.Encode()

	// Step 3: Display instructions and open browser
	fmt.Fprintf(os.Stdout, "Go to: %s\n", authorizeURL)
	if browser {
		if err := openBrowser(authorizeURL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open browser: %v (please open manually)\n", err)
		}
	}

	// Step 4: Start local server to capture authorization code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)
	server, err := startCallbackServer(codeChan, errChan)
	if err != nil {
		return fmt.Errorf("starting callback server: %w", err)
	}
	fmt.Println(server.Addr)

	// Wait for callback or timeout
	select {
	case code := <-codeChan:
		// Step 5: Exchange code for tokens
		tokenResp, err := exchangeCodeForToken(httpClient, code, codeVerifier)
		if err != nil {
			return fmt.Errorf("exchanging code for token: %w", err)
		}

		// Step 6: Save tokens
		if err := saveTokenFile(tokenResp.IDToken); err != nil {
			return fmt.Errorf("saving token file: %w", err)
		}

		return nil
	case err := <-errChan:
		return fmt.Errorf("callback server error: %w", err)
	case <-time.After(callbackTimeout):
		return fmt.Errorf("timed out waiting for authorization callback")
	}
}

// saveTokenFile saves the token response and orgID to ~/.kubiya.json.
func saveTokenFile(token string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting user home directory: %w", err)
	}
	filePath := filepath.Join(homeDir, ".kubiya.json")

	_ = os.RemoveAll(filePath)

	type TokenFile struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}

	data := TokenFile{
		Type:  "Bearer",
		Token: token,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token file: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	return nil
}

// startCallbackServer starts a local HTTP server to capture the authorization code.
func startCallbackServer(codeChan chan string, errChan chan error) (*http.Server, error) {
	// Check if port 9000 is available
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		return nil, fmt.Errorf("port 9000 is in use or unavailable: %w. Ensure the port is free or configure a different redirect URI in Auth0 and update redirectURI constant", err)
	}
	listener.Close() // Close immediately; we'll reopen in Serve

	server := &http.Server{Addr: ":9000"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if errorCode := query.Get("error"); errorCode != "" {
			errorDesc := query.Get("error_description")
			errChan <- fmt.Errorf("authorization failed: %s - %s. Check Auth0 Dashboard for valid organization name and application settings", errorCode, errorDesc)
			http.Error(w, fmt.Sprintf("Authorization failed: %s - %s", errorCode, errorDesc), http.StatusBadRequest)
			return
		}

		code := query.Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code in callback. Ensure the browser redirected to %s and the organization name is valid", auth0RedirectURI)
			http.Error(w, "No authorization code. Please try logging in again.", http.StatusBadRequest)
			return
		}

		codeChan <- code
		fmt.Fprintf(w, "Authentication successful! You can close this window.")
		// Shutdown server after handling callback
		go func() {
			time.Sleep(1 * time.Second)
			server.Shutdown(context.Background())
		}()
	})

	go func() {
		listener, err := net.Listen("tcp", ":9000")
		if err != nil {
			errChan <- fmt.Errorf("starting callback server: %w", err)
			return
		}
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	return server, nil
}

// exchangeCodeForToken exchanges the authorization code for tokens.
func exchangeCodeForToken(client *http.Client, code, codeVerifier string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("https://%s/oauth/token", auth0Domain)
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", auth0ClientId)
	data.Set("code", code)
	data.Set("redirect_uri", auth0RedirectURI)
	data.Set("code_verifier", codeVerifier)

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp TokenErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("parsing error response: %w", err)
		}
		return nil, fmt.Errorf("token exchange failed: %s - %s", errResp.Error, errResp.ErrorDescription)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &tokenResp, nil
}
