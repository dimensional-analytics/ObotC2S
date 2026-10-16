# Dimensional Analytics fork of obot-platform/providers

This fork adds the **PingFederate (OpenID Connect) auth provider** and an
overlay build that drops it onto the official Obot image:

- `pingfederate-auth-provider/`: the provider, its tests, and `hack/` tooling
  (a mock identity provider and a container end-to-end test). See its
  [README](pingfederate-auth-provider/README.md).
- `auth-providers/pingfederate-auth-provider.yaml`: the manifest Obot reads.
- `model-providers/generic-openai-model-provider.yaml`: declares the
  `OpenAIChatCompletions` dialect so agents use `/v1/chat/completions`.
- `patches/obot/pingfederate-profile.patch`: maps PingFederate's name into the
  Users page and adds email formatting to Users and My Account.
- `obot-overlay.Dockerfile`: builds `ghcr.io/jbittner-dimensional/obot:v0.26.3-pf.2`
  from the official image, replacing Obot with a rebuilt executable that
  embeds the patched UI, plus the provider and manifests above.

Build the overlay (the provider's tests run inside the build):

```sh
docker build -f obot-overlay.Dockerfile \
  --build-arg BASE_OBOT_IMAGE=ghcr.io/obot-platform/obot:v0.26.3 \
  -t ghcr.io/jbittner-dimensional/obot:v0.26.3-pf.2 .
```

To follow a new Obot release: fetch `https://github.com/obot-platform/providers`,
rebase this branch onto its `main`, rebase the Obot source patch and update
the pinned source commit and base image together, and run
`pingfederate-auth-provider/hack/e2e-container.sh`.

---

# Providers
The home of official Obot providers

`make build` produces static Go binaries with source paths, debug symbols, and
linker build IDs removed. Set `CGO_ENABLED=1` if a local build requires cgo.

The `providers` and `encryption-bins` Docker targets package UPX-compressed
binaries in `scratch` images. Obot consumes the existing `/obot-providers` and
`/bin/*-encryption-provider` paths with `COPY --from`; the images have no shell or
default command. A CA certificate bundle and writable `/tmp` are included for
running a binary directly with an explicit `--entrypoint`.

Compression minimizes executable size at the cost of build time and startup
decompression. To retain ordinary ELF binaries for debugging, binary inspection,
or environments that restrict executable unpacking, disable compression:

```sh
docker build --target providers --build-arg COMPRESS_BINARIES=false -t providers:uncompressed .
docker build --target encryption-bins --build-arg COMPRESS_BINARIES=false -t encryption-bins:uncompressed .
```

Docker builds cross-compile for the target platform, including `linux/amd64` and
`linux/arm64`, without running target binaries during the build. Local builds
remain uncompressed.

## Issues
Want to open an issue? Head over to the [Obot repo](https://github.com/obot-platform/obot/issues). Provider related issues will have the `providers` label.
