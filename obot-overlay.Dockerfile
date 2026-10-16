# syntax=docker/dockerfile:1
#
# Overlay for the official Obot image that adds the PingFederate auth provider
# and rebuilds Obot with the PingFederate profile/UI patch. Also declares
# the chat-completions dialect for the generic OpenAI model provider.
#
# The build context must be this repository's root so that the
# auth-providers-common replace directive in go.mod resolves:
#
#   docker build -f obot-overlay.Dockerfile \
#     --build-arg BASE_OBOT_IMAGE=ghcr.io/obot-platform/obot:v0.26.3 \
#     -t ghcr.io/jbittner-dimensional/obot:v0.26.3-pf.2 .
#
# The provider's test suite runs as part of the build; a failing test fails
# the image.
ARG BASE_OBOT_IMAGE=ghcr.io/obot-platform/obot:v0.26.3

FROM --platform=$BUILDPLATFORM cgr.dev/chainguard/wolfi-base AS build
RUN apk add --no-cache go-1.27 git ca-certificates
WORKDIR /src
COPY auth-providers-common /src/auth-providers-common
COPY pingfederate-auth-provider /src/pingfederate-auth-provider
WORKDIR /src/pingfederate-auth-provider
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go vet ./... && go test ./... && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w -buildid=" -o /out/obot-provider .

FROM --platform=$BUILDPLATFORM cgr.dev/chainguard/wolfi-base AS obot-build
RUN apk add --no-cache go-1.27 git nodejs npm pnpm patch ca-certificates
# The source patch targets this exact Obot release. Update source and base
# image together when rebasing the patch to a later release.
WORKDIR /src/obot
RUN git init && \
    git remote add origin https://github.com/obot-platform/obot.git && \
    git fetch --depth=1 origin 9324ea6d4feb4fac0febd95774c309e8a66b0e62 && \
    git checkout --detach FETCH_HEAD
COPY patches/obot/pingfederate-profile.patch /src/obot.patch
RUN patch --fuzz=0 -p1 < /src/obot.patch
WORKDIR /src/obot/ui/user
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile
RUN pnpm run check && pnpm run build
WORKDIR /src/obot
RUN gofmt -w apiclient/types/user.go pkg/gateway/types/users.go \
    pkg/gateway/client/user.go pkg/gateway/client/pingfederate_profile.go \
    pkg/gateway/client/pingfederate_profile_test.go
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go test ./pkg/gateway/client -run '^TestPingFederateProfilePreservesIdentity$' && \
    go test ./pkg/gateway/types
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -tags=kustomize_disable_go_plugin_support \
    -ldflags="-s -w -X github.com/obot-platform/obot/pkg/version.Tag=v0.26.3-pf.2" \
    -o /out/obot .

FROM ${BASE_OBOT_IMAGE}
LABEL ai.obot.overlay="pingfederate-auth-provider" \
      ai.obot.overlay.description="Adds PingFederate authentication, name claim mapping, email display domain, and the OpenAIChatCompletions dialect"
COPY --from=obot-build /out/obot /bin/obot
COPY --from=build /out/obot-provider /obot-providers/providers/pingfederate-auth-provider/bin/obot-provider
COPY auth-providers/pingfederate-auth-provider.yaml /obot-providers/providers/auth-providers/pingfederate-auth-provider.yaml
COPY model-providers/generic-openai-model-provider.yaml /obot-providers/providers/model-providers/generic-openai-model-provider.yaml
