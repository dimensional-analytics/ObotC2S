# PingFederate profile and email display

`pingfederate-profile.patch` targets Obot v0.26.3, commit
`9324ea6d4feb4fac0febd95774c309e8a66b0e62`. The overlay Dockerfile applies it
and rebuilds Obot, including the embedded UI. Changing the provider alone
does not enable this behavior in the official Obot executable.

The configuration UI also requires these entries under
`optionalConfigurationParameters` in
[`auth-providers/pingfederate-auth-provider.yaml`](../../auth-providers/pingfederate-auth-provider.yaml):

- `OBOT_PINGFEDERATE_AUTH_PROVIDER_NAME_CLAIM` (**Name Claim**)
- `OBOT_PINGFEDERATE_AUTH_PROVIDER_EMAIL_DISPLAY_DOMAIN` (**Email Display Domain**)

Both entries are included in the current manifest. When deploying, include the
updated YAML as well as the provider binary and patched Obot executable. The
overlay Dockerfile copies the manifest into the image. If either field is
missing from the UI, check the manifest in the running deployment; setting an
environment variable alone does not declare a UI field.

Configure **Identity & Access → Auth Providers → PingFederate**:

- **Name Claim**: the PingFederate attribute containing the user's display name.
  This populates **Users → Name**.
- **Email Claim**: the existing claim used for account matching, e.g. `uid`.
- **Email Display Domain**: e.g. `company.com`. An email claim of `JDOE` then
  renders as `jdoe@company.com` in Users and My Account. Full addresses don't
  receive a second suffix. Users search recognizes the displayed address.

Raw email, username, subject, hashes, roles, and group mappings stay unchanged.
Other users without a display domain retain their existing email rendering.

Obot automatically adds a `users.email_display_domain` column at startup.
Existing profiles refresh within five minutes of authenticated activity.
Removing the setting clears the stored display domain on the next refresh.
Reverting to an older image leaves the unused column and updated display names
in the database.

The build runs provider vet/tests, UI type checking and a production build,
and backend tests for preserving identity. To run the included browser test
against a patched Obot source checkout, from `ui/user` run:

```sh
pnpm exec playwright install chromium
pnpm exec vitest run src/lib/components/profile/DisplayEmail.svelte.spec.ts --browser.headless
```
