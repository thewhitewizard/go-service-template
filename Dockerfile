# Multi-stage build: the toolchain never reaches the final image.
#
# Base images are pinned by digest, not by tag alone. A tag is mutable — today's
# golang:1.26-alpine is not tomorrow's — so a tag-only reference makes a build
# that succeeded once unreproducible, and a registry-side substitution invisible.
# The tag is kept alongside the digest purely for human readability.
#
# Refresh a digest deliberately, in a commit of its own:
#   docker buildx imagetools inspect --raw golang:1.26-alpine | sha256sum
#   docker buildx imagetools inspect golang:1.26-alpine       # shows the index digest
FROM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build

WORKDIR /src

# Dependencies first: this layer stays cached until go.mod/go.sum actually change,
# so a source-only edit does not re-download the module graph.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary, which is what allows the final image to
# be distroless with no libc at all.
# -trimpath strips local filesystem paths from the binary; without it the image
# ships the build machine's directory layout.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/service ./cmd/service

# distroless/static: no shell, no package manager, no libc. Nothing to pivot to if
# the process is ever compromised.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35

# The nonroot variant already runs as uid 65532, but declaring it makes the
# guarantee explicit and survives a base image swap.
USER 65532:65532

COPY --from=build /out/service /service

EXPOSE 8080

# Exec form, by necessity as much as by choice: there is no shell to wrap it. The
# binary is PID 1 and receives SIGTERM directly, which is what makes the graceful
# shutdown in cmd/service/main.go actually run. Wrapping it in a shell would swallow
# the signal and every deploy would end in a 30-second kill.
ENTRYPOINT ["/service"]
