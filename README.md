# Kubernetes Home Server Portal


Small App that checks all available ingress applications and displays a portal with one tile per application. Clicking on a tile opens the corresponding ingress url.

## Features:

- Display name and description of the app.
- Display image using base64 or http url.
- Auto update without requiring restarts.
- OIDC via oauth2 proxy.
- Filter displayed tiles according to SSO groups.

## Troubleshooting

- Error invalid CSRF cookie and redirect loop issues: cookie must have a different name since *.example.com already has
  an oauth2-proxy