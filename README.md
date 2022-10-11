# keycloak-ext-operator

Creates OAuth clients in Keycloak and creates corresponding secrets in kubernetes.

Required environment:

| Environment variable  | Purpose                           |
|-----------------------|-----------------------------------|
| `KEYCLOAK_URL`        | URL to keycloak instance          |
| `KEYCLOAK_USER`       | Admin user name (usually `admin`) |
| `KEYCLOAK_PASSWORD`   | Admin password                    |

By default, those values will be obtained from secret `keycloak` in `system` namespace.

## Description

The operator:

- watches `KeycloakClient` manifests
- creates (or updates) OAuth private clients in Keycloak instance. If it's a new client, then secret will be randomly
  generated
- creates secret with OAuth credentials

Tested on Keycloak 19. May not work on versions bellow 18 due to different API URLs.

**Example:**

Manifest (CRD)

```yaml
apiVersion: keycloak.k8s.reddec.net/v1alpha1
kind: KeycloakClient
metadata:
  name: sample
  namespace: default
spec:
  secretName: "my-secret"
  domain: "example.com"
  realm: reddec
```

Generated secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oauth
  namespace: default
immutable: true
type: Opaque
data:
  clientID: .....     # unless copied from existent, it's equal to domain name
  clientSecret: ..... # automatically generated secret (32 crypto random bytes represented as 64-bytes hex) or copied from existent client definition from keycloak.
  realm: .....        # copied from spec
  realmURL: .....     # full URL to realm: <keycloak url>/realms/<realm>
  discoveryURL: ..... # OIDC URL to realm: <keycloak url>/realms/<realm>/.well-known/openid-configuration
```

* unless `clientSecret` is copied from existent Keycloak client, it is automatically generated secret from 32 crypto
  random bytes, and represented as 64-bytes hex

## Getting Started

* Install operator
* Setup credentials

## License

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

