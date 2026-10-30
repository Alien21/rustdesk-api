# API build outputs

For the combined API/admin build, use the sibling RustDesk workspace launcher: `bash build_linux.sh --only api` or `bash build_windows.sh --only api`, with `--check` for a preflight. The launcher installs frontend dependencies only in a staged copy and packages the generated admin assets with the API.

For a standalone backend build, run `./build.sh` on Linux/macOS or `build.bat` on Windows with Go 1.23.10 or newer and a C compiler for the selected target on PATH. The default target is the current host; set `GOOS`, `GOARCH` and, when needed, `CC`/`GOARM` for a cross build. CGO stays enabled by default for SQLite.

Both workflows treat this checkout as an input. The default shared base is its sibling `../artifacts/`, with these paths relative to that base:

- `rustdesk-api/build/<goos>-<arch>/`: isolated copy of the current working tree, including uncommitted changes, used for Go and Swagger generation.
- `rustdesk-api/publish/<goos>-<arch>/`: application binary, resources, documentation, configuration and empty data/runtime directories.
- `tools/rustdesk-api/go-build/`, `tools/rustdesk-api/go-mod/`, `tools/rustdesk-api/gopath/`: Go build/module/checksum caches.
- `tools/rustdesk-api/linux-tools/` and `tools/rustdesk-api/go-toolchain/go/`: managed Linux and Windows Go SDKs.
- `tmp/<host-platform>/rustdesk-api/`: temporary files; the usual host platforms are `linux-x64` and `windows-x64`.

Project artifact roots contain only `build/` and `publish/`. Our helper scripts, harnesses, logs and reports belong under `artifacts/AI/`. Direct low-level Go/npm commands belong only in disposable working copies under artifacts.

The wrappers scope environment changes to the build process; they do not run `go env -w` or automatically install another Go toolchain. `DOCS=auto` regenerates Swagger when `swag` is on PATH and otherwise uses checked-in documentation. `DOCS=true` requires `swag`; `DOCS=false` always uses existing documentation. Generation and dependency metadata changes happen in the isolated stage. The helper uses only the Go standard library.

The companion admin frontend is published to `../artifacts/rustdesk-api-web/publish/web/`. The API helper automatically includes it when it contains `index.html`. Set `WEB_DIST` to an explicit frontend output directory to override it. Without one, the API's existing resource files are packaged.

For example:

```sh
GOOS=linux GOARCH=amd64 DOCS=false ./build.sh
WEB_DIST=../artifacts/rustdesk-api-web/publish/web ./build.sh
```

Run the application from the published directory so its relative resource/configuration paths work. Build/publish directories are disposable; keep deployment data outside them.

## CI and Docker

Release workflows use `artifacts/build/source/` for Swagger/Go source staging and `artifacts/build/release/` for archive contents. ZIP/TAR files are written under `artifacts/publish/` and retain the existing internal `./release` directory. Downstream Debian and Docker jobs extract into `artifacts/build/<arch>/release/`; Debian packaging also stays under `artifacts/`.

`Dockerfile` and `Dockerfile_full_s6` consume the extracted release for `BUILDARCH`. Build them from the API repository root after extracting a release archive to `artifacts/build/<arch>/`.

`Dockerfile.dev` and the release workflows default to `Alien21/rustdesk-api-web`, branch `develop-alien21`, output directory `artifacts/publish/web`. Push that frontend branch before running a remote build so the current security fixes are included. Local API builds use the sibling frontend output directly. Docker's `FRONTEND_GIT_REPO`, `FRONTEND_GIT_BRANCH` and `FRONTEND_DIST` arguments allow another checkout; an unmodified upstream frontend uses branch `master` and output `dist`.

## Shared workspace artifacts

The workspace launcher sets `RUSTDESK_ARTIFACTS_DIR` to an absolute central base, normally `C:/Git/RustDesk/artifacts` (WSL: `/mnt/c/Git/RustDesk/artifacts`). The API then uses:

- `<base>/rustdesk-api/build/<goos>-<arch>/` for source staging.
- `<base>/rustdesk-api/publish/<goos>-<arch>/` for the application bundle.
- `<base>/tools/rustdesk-api/` for Go SDKs and caches.
- `<base>/tmp/<host-platform>/rustdesk-api/` for temporary files.

The helper also discovers the frontend at `<base>/rustdesk-api-web/publish/web/`. An explicit `WEB_DIST` still takes precedence. The central API namespace must be separate from the source checkout; redirected artifact directories are rejected.

For a standalone Linux build with an existing Go/C toolchain:

```sh
RUSTDESK_ARTIFACTS_DIR=/mnt/c/Git/RustDesk/artifacts ./build.sh
```

With this environment variable unset, the same layout uses the sibling `../artifacts/` base. CI and Docker choose their own disposable staging directories and explicit frontend output paths. Selecting a root does not migrate or remove old artifacts.

## Executable name and bundle contents

The executable is `rustdesk-api` on Linux/macOS and `rustdesk-api.exe` on
Windows. The Go source entrypoint remains `cmd/apimain.go`. Docker, release
workflows and local workspace launchers use the same executable name.

| Bundle path | Purpose | Updating an existing installation |
| --- | --- | --- |
| `rustdesk-api` / `rustdesk-api.exe` | API server executable | Replace the installed executable while its service is stopped. |
| `resources/admin/` | Compiled web administration, served at `/_admin/` | Replace this frontend directory as a complete set to avoid mixing asset versions. |
| `resources/i18n/`, `resources/templates/`, `resources/version` | Runtime translations, HTML templates and version metadata | Keep them available; update these supplied files when they differ from the installed version. |
| `resources/web/` | Optional browser remote desktop client | Needed only with the corresponding web-client feature enabled. |
| `resources/public/` | Public resources, including persistent uploads on some installations | Preserve existing user files. |
| `conf/` | Sample server configuration for a new installation | Preserve the server's existing configuration and secrets. |
| `data/` | Empty initial directory for persistent application/database data | Preserve the server's existing data. |
| `runtime/` | Empty initial directory for logs and file caches | Preserve the existing directory; the application needs write access. |
| `docs/` | Swagger source/export files | Not required on disk: the generated API/admin Swagger definitions are compiled into the executable. |

Uploaded files may also exist under `resources/upload/`, depending on the
handler/version. Preserve both that directory and `resources/public/upload/`
when updating resources; do not synchronize the whole bundle with deletion
enabled over a live installation.

For the current Alien21 server layout, the executable is
`/usr/bin/rustdesk-api` and the working directory is `/var/lib/rustdesk-api/`.
Admin assets therefore belong in `/var/lib/rustdesk-api/resources/admin/`.
Copying the sample `conf/`, empty `data/` or empty `runtime/` is unnecessary
for an existing configured server.

The installed server was compared with this bundle on 2026-10-22: all seven
i18n files, both OAuth HTML templates and `resources/version` have the same
content (the text files differ only in line endings). For this update, only
the executable and `resources/admin/` need replacement.

Regression surface of the executable rename:

- `scripts/build/main.go` selects the new output filename.
- `cmd/apimain.go` changes only the CLI help command name.
- `Dockerfile`, `Dockerfile.dev`, `Dockerfile_full_s6`,
  `.github/workflows/build.yml` and `.github/workflows/build_test.yml` consume
  that filename when packaging or starting the API.
- The sibling client's `scripts/build/api.ps1` and `scripts/build/api.sh`
  check for that filename after the Windows/Linux build.
- `README.md`, `README_EN.md` and this `README.build.md` document the matching
  launch commands and update contents.

Each consumer must use the produced filename to keep its build/start path
working. HTTP handlers and configuration formats are unchanged.
