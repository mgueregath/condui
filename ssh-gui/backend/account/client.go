package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type apiClient struct {
	serverURL   string
	accessToken string
	httpClient  *http.Client
}

func newAPIClient(serverURL, accessToken string) *apiClient {
	return &apiClient{
		serverURL:   serverURL,
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *apiClient) do(method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.serverURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		if errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("server error %d", resp.StatusCode)
	}

	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func (c *apiClient) register(email, password string) error {
	return c.do("POST", "/api/v1/auth/register", registerRequest{Email: email, Password: password}, nil)
}

func (c *apiClient) login(email, password, deviceName string) (*loginResponse, error) {
	var resp loginResponse
	err := c.do("POST", "/api/v1/auth/login", loginRequest{
		Email: email, Password: password, DeviceName: deviceName,
	}, &resp)
	return &resp, err
}

func (c *apiClient) refreshToken(refreshToken string) (string, error) {
	var resp refreshResponse
	err := c.do("POST", "/api/v1/auth/refresh", refreshRequest{RefreshToken: refreshToken}, &resp)
	return resp.AccessToken, err
}

func (c *apiClient) logout(refreshToken string) error {
	return c.do("POST", "/api/v1/auth/logout", map[string]string{"refreshToken": refreshToken}, nil)
}

func (c *apiClient) getMe() (*userPayload, error) {
	var user userPayload
	err := c.do("GET", "/api/v1/auth/me", nil, &user)
	return &user, err
}

func (c *apiClient) updateIdentity(pubKey, identityBlob string) error {
	return c.do("PUT", "/api/v1/auth/identity", identityRequest{
		PublicKey: pubKey, IdentityBlob: identityBlob,
	}, nil)
}

func (c *apiClient) listBlobs() ([]BlobMeta, error) {
	var blobs []BlobMeta
	err := c.do("GET", "/api/v1/blobs", nil, &blobs)
	return blobs, err
}

func (c *apiClient) getBlob(id string) (*BlobPayload, error) {
	var blob BlobPayload
	err := c.do("GET", "/api/v1/blobs/"+id, nil, &blob)
	return &blob, err
}

func (c *apiClient) getBlobMeta(id string) (*BlobMeta, error) {
	var meta BlobMeta
	err := c.do("GET", "/api/v1/blobs/"+id+"/meta", nil, &meta)
	return &meta, err
}

func (c *apiClient) upsertBlob(blob BlobPayload) error {
	// Try PUT first (update), fall back to POST (create)
	err := c.do("PUT", "/api/v1/blobs/"+blob.ID, blob, nil)
	if err != nil {
		// If blob doesn't exist yet, create it
		return c.do("POST", "/api/v1/blobs", blob, nil)
	}
	return nil
}

func (c *apiClient) deleteBlob(id string) error {
	return c.do("DELETE", "/api/v1/blobs/"+id, nil, nil)
}

func (c *apiClient) createShare(req shareRequest) (*shareResponse, error) {
	var resp shareResponse
	err := c.do("POST", "/api/v1/shares", req, &resp)
	return &resp, err
}

func (c *apiClient) getReceivedShares() ([]ShareInfo, error) {
	var shares []ShareInfo
	err := c.do("GET", "/api/v1/shares/received", nil, &shares)
	return shares, err
}

func (c *apiClient) getSentShares() ([]ShareInfo, error) {
	var shares []ShareInfo
	err := c.do("GET", "/api/v1/shares/sent", nil, &shares)
	return shares, err
}

func (c *apiClient) acceptShare(shareID string) error {
	return c.do("PUT", "/api/v1/shares/"+shareID+"/accept", nil, nil)
}

func (c *apiClient) deleteShare(shareID string) error {
	return c.do("DELETE", "/api/v1/shares/"+shareID, nil, nil)
}

func (c *apiClient) getRecipientPublicKey(email string) (string, error) {
	var resp struct {
		PublicKey string `json:"publicKey"`
	}
	err := c.do("GET", "/api/v1/users/public-key?email="+url.QueryEscape(email), nil, &resp)
	return resp.PublicKey, err
}
