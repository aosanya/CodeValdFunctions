> ⚠️ **DEPRECATED** — This file was copied from CodeValdGit and describes git service internals that do not apply to CodeValdFunctions. Retained for reference only. See [architecture.md](architecture.md) for the correct CodeValdFunctions architecture.

---

# CodeValdFunctions — Git Smart HTTP Transport

## Overview

CodeValdFunctions serves the [Git Smart HTTP protocol](https://git-scm.com/docs/http-protocol)
alongside its gRPC service so that standard `git clone`, `git fetch`, and `git push`
clients can interact with agency repositories directly.

---

## `plumbing/transport` — Core Transport Interfaces

**Import path**: `github.com/go-git/go-git/v5/plumbing/transport`

This package defines the language-neutral contracts that all go-git transport
implementations (HTTP, SSH, file, git://) must satisfy.

### Key types

| Type | Purpose |
|---|---|
| `Transport` | Factory that creates upload-pack and receive-pack sessions for a given endpoint |
| `UploadPackSession` | Handles `git fetch` / `git clone` — advertise refs and stream a pack file to the client |
| `ReceivePackSession` | Handles `git push` — advertise refs and accept a pack file from the client |
| `Endpoint` | Parsed Git URL; the `Path` field (e.g. `"/agency-42"`) is used as the repository key |
| `AuthMethod` | Optional authentication credential (pass `nil` for unauthenticated access) |

### Interface signatures

```go
type Transport interface {
    NewUploadPackSession(*Endpoint, AuthMethod) (UploadPackSession, error)
    NewReceivePackSession(*Endpoint, AuthMethod) (ReceivePackSession, error)
}

type UploadPackSession interface {
    AdvertisedReferencesContext(context.Context) (*packp.AdvRefs, error)
    UploadPack(context.Context, *packp.UploadPackRequest) (*packp.UploadPackResponse, error)
    io.Closer
}

type ReceivePackSession interface {
    AdvertisedReferencesContext(context.Context) (*packp.AdvRefs, error)
    ReceivePack(context.Context, *packp.ReferenceUpdateRequest) (*packp.ReportStatus, error)
    io.Closer
}
```

### Service-name constants

```go
const (
    UploadPackServiceName  = "git-upload-pack"   // fetch / clone
    ReceivePackServiceName = "git-receive-pack"  // push
)
```

---

## `plumbing/transport/server` — Server-Side Transport Engine

**Import path**: `github.com/go-git/go-git/v5/plumbing/transport/server`

Turns a `Loader` into a `transport.Transport` that an HTTP handler can call.

### `Loader` interface

```go
type Loader interface {
    Load(ep *transport.Endpoint) (storer.Storer, error)
}
```

CodeValdFunctions provides a custom `backendLoader` that maps `ep.Path` → agencyID:

```go
type backendLoader struct{ b codevaldgit.Backend }

func (l *backendLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
    agencyID := strings.Trim(ep.Path, "/")
    sto, _, err := l.b.OpenStorer(context.Background(), agencyID)
    if err != nil {
        return nil, transport.ErrRepositoryNotFound
    }
    return sto, nil
}
```

### Built-in loader variants

| Constructor | Behaviour |
|---|---|
| `NewFilesystemLoader(base billy.Filesystem)` | Resolves `ep.Path` as a sub-path under `base` |
| `MapLoader` (`map[string]storer.Storer`) | Directly maps endpoint string → storer; useful for tests |

### `NewServer`

```go
func NewServer(loader Loader) transport.Transport
```

Stateless, safe to share across goroutines. One instance constructed at startup.

---

## `plumbing/protocol/packp` — Pack Protocol Messages

**Import path**: `github.com/go-git/go-git/v5/plumbing/protocol/packp`

### Types used in GIT-007

| Type | Direction | Used in |
|---|---|---|
| `AdvRefs` | server → client | Both `info/refs` endpoints |
| `UploadPackRequest` | client → server | `POST /{agencyID}/git-upload-pack` body |
| `UploadPackResponse` | server → client | `POST /{agencyID}/git-upload-pack` response |
| `ReferenceUpdateRequest` | client → server | `POST /{agencyID}/git-receive-pack` body |
| `ReportStatus` | server → client | `POST /{agencyID}/git-receive-pack` response |

### `AdvRefs.Prefix` — Smart HTTP service advertisement

```go
advRefs.Prefix = [][]byte{
    []byte("# service=" + transport.UploadPackServiceName),
    pktline.Flush,
}
```

`pktline.Flush` is the sentinel that the encoder translates to the flush packet `0000`.

### Encode / Decode pattern

```go
req := packp.NewUploadPackRequest()
if err := req.Decode(r.Body); err != nil { ... }

if err := resp.Encode(w); err != nil { ... }
```

---

## `plumbing/format/pktline` — Packet-Line Framing

**Import path**: `github.com/go-git/go-git/v5/plumbing/format/pktline`

| Symbol | Purpose |
|---|---|
| `Flush` (`[]byte`) | Sentinel for flush packet `0000` |
| `NewEncoder(w io.Writer)` | Writes pkt-line framed data |
| `NewScanner(r io.Reader)` | Reads pkt-line framed data |
| `Encoder.Encodef(format, args...)` | Printf-style pkt-line write |
| `Encoder.Flush()` | Write flush packet `0000` |

In GIT-007 `pktline` is used indirectly via `packp.AdvRefs.Prefix`; the handler does
not call `pktline.NewEncoder` directly.

---

## `github.com/soheilhy/cmux` — gRPC + HTTP on One Port

**Import path**: `github.com/soheilhy/cmux`

cmux inspects the first bytes of each incoming TCP connection and dispatches to a
matching `net.Listener`, allowing gRPC (HTTP/2) and plain HTTP/1.1 (Git Smart HTTP)
to share a **single listen port**.

### Matching rules

```go
m := cmux.New(lis)

grpcL := m.MatchWithWriters(
    cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"),
)

httpL := m.Match(cmux.Any())

go grpcServer.Serve(grpcL)
go http.Serve(httpL, gitHTTPHandler)
go m.Serve()
```

---

## Smart HTTP Endpoint Reference

The `GitHTTPHandler` (`internal/server/githttp.go`) registers four routes:

| Method | Path pattern | Service | Content-Type (response) |
|---|---|---|---|
| `GET` | `/{agencyID}/info/refs?service=git-upload-pack` | Upload-pack advertisement | `application/x-git-upload-pack-advertisement` |
| `GET` | `/{agencyID}/info/refs?service=git-receive-pack` | Receive-pack advertisement | `application/x-git-receive-pack-advertisement` |
| `POST` | `/{agencyID}/git-upload-pack` | Pack transfer (clone/fetch) | `application/x-git-upload-pack-result` |
| `POST` | `/{agencyID}/git-receive-pack` | Pack transfer (push) | `application/x-git-receive-pack-result` |

All responses include `Cache-Control: no-cache`.

### `info/refs` response body format

```
<pkt-line "# service=git-upload-pack
">
<flush-pkt "0000">
<AdvRefs encoded as pkt-lines>
```

---

## Library Version Summary

| Library | Version | Role |
|---|---|---|
| `github.com/go-git/go-git/v5` | v5.16.5 | Git engine — all operations |
| `github.com/go-git/go-git/v5/plumbing/transport` | (bundled) | Transport interfaces |
| `github.com/go-git/go-git/v5/plumbing/transport/server` | (bundled) | Server-side transport engine |
| `github.com/go-git/go-git/v5/plumbing/protocol/packp` | (bundled) | Pack protocol message types |
| `github.com/go-git/go-git/v5/plumbing/format/pktline` | (bundled) | Pkt-line framing |
| `github.com/go-git/go-billy/v5` | v5.8.0 | Working-tree filesystem abstraction |
| `github.com/soheilhy/cmux` | TBD (added in GIT-009) | TCP multiplexer |
