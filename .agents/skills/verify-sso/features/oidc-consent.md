# OIDC consent

A relying party starts a PKCE authorization request, the user grants access on
the consent screen, and the browser returns to the client's redirect URI with an
authorization code. The client then exchanges the code for tokens and reads the
user's claims.

## Sub-features

- `oidc-discovery` serves `/.well-known/openid-configuration` and `/jwks` for the realm.
- `oidc-authorize` validates client, redirect URI, scopes, and PKCE, then routes to sign-in or consent.
- `oidc-consent-grant` shows `Grant access` and returns the callback with `code` and the original `state`.
- `oidc-consent-remember` skips the consent screen on a later request for the same scopes.
- `oidc-token` exchanges the code, verifier, and redirect URI for access, ID, and refresh tokens.
- `oidc-userinfo` returns claims allowed by the granted scopes.
- `oidc-logout` ends the session and redirects a registered post-logout URI.

## How to get to it (user POV)

- The client sends the browser to
  `/realms/default/authorize?response_type=code&client_id=sample-spa&redirect_uri=http://localhost:5173/callback&scope=openid profile email offline_access&state=...&nonce=...&code_challenge=...&code_challenge_method=S256`.
- A signed-out user signs in first; a third-party client then shows `/consent`.
- `Allow access` returns to the client's `redirect_uri` with `code` and `state`.
- The client calls `POST /realms/default/token`, then `GET /realms/default/userinfo`.

## Driving it with Playwright

Preconditions:

- Stack healthy at `http://localhost:8093`.
- `sample-spa` exists and is third-party; `scripts/e2e-stack.sh` sets
  `third_party = true` for it.
- The member is signed in (`storageState: userStatePath`).

- **Consent and callback.** Run `cd web && E2E_BASE_URL=http://localhost:8093 pnpm exec playwright test e2e/consent.spec.ts --trace on`. The spec routes `http://localhost:5173/**`, opens the authorize URL, handles either `Grant access` or `Continue`, and asserts the callback origin, path, `state`, and a truthy `code`.
- **Token exchange.** In a temporary spec, keep the spec's `state` and `verifier` in variables, capture `callbackURL` the same way, then exchange the code:
  ```ts
  const response = await page.request.post('http://localhost:8093/realms/default/token', {
    form: {
      grant_type: 'authorization_code',
      code: new URL(callbackURL!).searchParams.get('code')!,
      redirect_uri: 'http://localhost:5173/callback',
      client_id: 'sample-spa',
      code_verifier: verifier,
    },
  })
  expect(response.status()).toBe(200)
  const tokens = await response.json()
  expect(tokens.access_token).toBeTruthy()
  expect(tokens.id_token).toBeTruthy()
  ```
  The response is `application/json` with `access_token`, `token_type`, `expires_in`, `id_token`, `refresh_token`, and `scope`.
- **Userinfo.** Send `GET /realms/default/userinfo` with `Authorization: Bearer <access_token>`; the body contains the user's `sub` and `email` for the granted scopes.
- **Consent remembered.** Repeat the authorize request with the same client and scopes; the browser lands on the callback without showing `Grant access`.
- **Proof.** Capture the callback URL with its `state` and `code`, the token response status `200`, and the userinfo body; keep the trace.

## Gotchas

- `redirect_uri` must be registered for the client. `http://localhost:5173/callback` is seeded; any other value returns `400 redirect_uri is not registered for this client`.
- PKCE with `S256` is mandatory; a missing or `plain` challenge returns an OAuth error.
- The `openid` scope is required, and every requested scope must be allowed for the client. `offline_access` needs the client's refresh-token grant.
- Consent is stored per user and client. After the first grant, the same scopes skip the consent screen; revoke it or use a different user to see the screen again.
- A dev database (`make dev`) leaves `sample-spa` first-party, so the consent screen never appears there. Only the e2e stack flips it.
- Start the authorize flow from a browser context. A session cookie obtained by scripting `POST /api/v1/auth/login` with curl was not accepted by `/realms/default/authorize` during setup; the browser path is the supported one.
- `prompt=none` returns `login_required` or `interaction_required` as a redirect error instead of a code when the session or consent is missing.
