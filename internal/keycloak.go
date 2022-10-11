/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

func FromEnv() (*Keycloak, error) {
	var cfg Keycloak
	return &cfg, envconfig.Process("KEYCLOAK", &cfg)
}

type Keycloak struct {
	URL      string `required:"true" envconfig:"URL"`
	User     string `required:"true" envconfig:"USER"`
	Password string `required:"true" envconfig:"PASSWORD"`
}

func (k *Keycloak) RealmURL(realm string) string {
	return strings.TrimRight(k.URL, "/") + `/realms/` + url.PathEscape(realm)
}

func (k *Keycloak) DiscoveryURL(realm string) string {
	return k.RealmURL(realm) + "/.well-known/openid-configuration"
}

type ClientDraft struct {
	ClientID     string   `json:"clientId,omitempty"`
	ClientSecret string   `json:"secret,omitempty"`
	RootURL      string   `json:"rootUrl,omitempty"`
	AdminURL     string   `json:"adminUrl,omitempty"`
	RedirectURIs []string `json:"redirectUris,omitempty"`
	WebOrigins   []string `json:"webOrigins,omitempty"`
	Name         string   `json:"name,omitempty"`
	ID           string   `json:"id,omitempty"`
	Description  string   `json:"description,omitempty"`
}

func Generate(domain string) ClientDraft {
	var key [32]byte
	_, err := io.ReadFull(rand.Reader, key[:])
	if err != nil {
		panic(err)
	}
	secret := hex.EncodeToString(key[:])
	clientURL := "https://" + domain
	return ClientDraft{
		ClientID:     domain,
		ClientSecret: secret,
		RootURL:      clientURL,
		AdminURL:     clientURL,
		RedirectURIs: []string{
			clientURL + "/*",
		},
		WebOrigins: []string{
			clientURL,
		},
		Name: domain,
	}
}

type AuthorizedKeycloak struct {
	config Keycloak
	token  string
	err    error
}

func (k *AuthorizedKeycloak) Error() error {
	return k.err
}

func (k *AuthorizedKeycloak) Delete(ctx context.Context, realm, id string) error {
	if k.err != nil {
		return k.err
	}
	href := strings.TrimRight(k.config.URL, "/") + `/admin/realms/` + url.PathEscape(realm) + `/clients/` + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, href, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", k.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("status: %d", res.StatusCode)
	}
	return nil
}

func (k *AuthorizedKeycloak) Update(ctx context.Context, id string, realm string, draft ClientDraft) error {
	if k.err != nil {
		return k.err
	}
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(draft); err != nil {
		return fmt.Errorf("encode payload, %w", err)
	}
	href := strings.TrimRight(k.config.URL, "/") + `/admin/realms/` + url.PathEscape(realm) + `/clients/` + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, href, &data)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", k.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("status: %d", res.StatusCode)
	}
	return nil
}

// Create new client and return ID.
func (k *AuthorizedKeycloak) Create(ctx context.Context, realm string, draft ClientDraft) (string, error) {
	if k.err != nil {
		return "", k.err
	}
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(draft); err != nil {
		return "", fmt.Errorf("encode payload, %w", err)
	}
	href := strings.TrimRight(k.config.URL, "/") + `/admin/realms/` + url.PathEscape(realm) + `/clients`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, href, &data)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", k.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("status: %d", res.StatusCode)
	}
	id := path.Base(res.Header.Get("Location"))
	return id, nil
}

func (k *AuthorizedKeycloak) Clients(ctx context.Context, realm string) *Clients {
	if k.err != nil {
		return &Clients{err: k.err}
	}
	href := strings.TrimRight(k.config.URL, "/") + `/admin/realms/` + url.PathEscape(realm) + `/clients`
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, href, nil)
	if err != nil {
		return &Clients{err: fmt.Errorf("create request: %w", err)}
	}
	req.Header.Set("Authorization", k.token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return &Clients{err: fmt.Errorf("do request: %w", err)}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return &Clients{err: fmt.Errorf("status: %d", res.StatusCode)}
	}

	var ans []Client
	return &Clients{list: ans, err: json.NewDecoder(res.Body).Decode(&ans)}
}

func (k *AuthorizedKeycloak) Get(ctx context.Context, realm string, id string) (*ClientDetails, error) {
	if k.err != nil {
		return nil, k.err
	}
	href := strings.TrimRight(k.config.URL, "/") + `/admin/realms/` + url.PathEscape(realm) + `/clients/` + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, href, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", k.token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrClientNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", res.StatusCode)
	}
	var ans ClientDetails
	return &ans, json.NewDecoder(res.Body).Decode(&ans)
}

func (k *Keycloak) Authorize(ctx context.Context) *AuthorizedKeycloak {
	var form = url.Values{
		"grant_type": []string{"password"},
		"client_id":  []string{"admin-cli"},
		"username":   []string{k.User},
		"password":   []string{k.Password},
	}
	href := strings.TrimRight(k.URL, "/") + `/realms/master/protocol/openid-connect/token`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, href, strings.NewReader(form.Encode()))
	if err != nil {
		return &AuthorizedKeycloak{err: fmt.Errorf("create request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return &AuthorizedKeycloak{err: fmt.Errorf("do request: %w", err)}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return &AuthorizedKeycloak{err: fmt.Errorf("status: %d", res.StatusCode)}
	}

	var token struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		return &AuthorizedKeycloak{err: fmt.Errorf("decode result: %w", err)}
	}
	return &AuthorizedKeycloak{
		config: *k,
		token:  token.TokenType + " " + token.AccessToken,
	}
}

var ErrClientNotFound = errors.New("client not found")

type Clients struct {
	list []Client
	err  error
}

func (cl *Clients) Error() error {
	return cl.err
}

func (cl *Clients) All() ([]Client, error) {
	return cl.list, cl.err
}

func (cl *Clients) Find(clientID string) (Client, error) {
	if cl.err != nil {
		return Client{}, cl.err
	}
	for _, it := range cl.list {
		if it.ClientID == clientID {
			return it, nil
		}
	}
	return Client{}, ErrClientNotFound
}

func (cl *Clients) ByName(name string) (Client, error) {
	if cl.err != nil {
		return Client{}, cl.err
	}
	for _, it := range cl.list {
		if it.Name == name {
			return it, nil
		}
	}
	return Client{}, ErrClientNotFound
}

func Find(ctx context.Context, kClient *AuthorizedKeycloak, realm string, id, name string) (*ClientDetails, error) {
	// check by ID
	existent, err := kClient.Get(ctx, realm, id)
	if err == nil {
		return existent, nil
	}
	if !errors.Is(err, ErrClientNotFound) {
		return nil, fmt.Errorf("get client: %w", err)
	}

	// check by Name
	item, err := kClient.Clients(ctx, realm).ByName(name)
	if err == nil {
		return kClient.Get(ctx, realm, item.ID)
	}
	if !errors.Is(err, ErrClientNotFound) {
		return nil, fmt.Errorf("list clients: %w", err)
	}

	return nil, ErrClientNotFound
}
