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

type Client struct {
	ID                        string   `json:"id"`
	ClientID                  string   `json:"clientId"`
	Name                      string   `json:"name"`
	Description               string   `json:"description,omitempty"`
	AdminURL                  string   `json:"adminUrl,omitempty"`
	RootURL                   string   `json:"rootUrl"`
	BaseURL                   string   `json:"baseUrl"`
	SurrogateAuthRequired     bool     `json:"surrogateAuthRequired"`
	Enabled                   bool     `json:"enabled"`
	AlwaysDisplayInConsole    bool     `json:"alwaysDisplayInConsole"`
	ClientAuthenticatorType   string   `json:"clientAuthenticatorType"`
	RedirectURIs              []string `json:"redirectUris"`
	WebOrigins                []string `json:"webOrigins"`
	NotBefore                 int      `json:"notBefore"`
	BearerOnly                bool     `json:"bearerOnly"`
	ConsentRequired           bool     `json:"consentRequired"`
	StandardFlowEnabled       bool     `json:"standardFlowEnabled"`
	ImplicitFlowEnabled       bool     `json:"implicitFlowEnabled"`
	DirectAccessGrantsEnabled bool     `json:"directAccessGrantsEnabled"`
	ServiceAccountsEnabled    bool     `json:"serviceAccountsEnabled"`
	PublicClient              bool     `json:"publicClient"`
	FrontChannelLogout        bool     `json:"frontchannelLogout"`
	Protocol                  string   `json:"protocol"`
	Attributes                struct {
		PostLogoutRedirectUris string `json:"post.logout.redirect.uris"`
	} `json:"attributes"`
	FullScopeAllowed          bool     `json:"fullScopeAllowed"`
	NodeReRegistrationTimeout int      `json:"nodeReRegistrationTimeout"`
	DefaultClientScopes       []string `json:"defaultClientScopes"`
	OptionalClientScopes      []string `json:"optionalClientScopes"`
	Access                    struct {
		View      bool `json:"view"`
		Configure bool `json:"configure"`
		Manage    bool `json:"manage"`
	} `json:"access"`
}

type ClientDetails struct {
	Client
	Secret string `json:"secret"`
}
