# API build outputs

Run `./build.sh` on Linux/macOS or `build.bat` on Windows with Go and a C compiler for the selected target on PATH. The default target is the current host; set `GOOS`, `GOARCH` and, when needed, `CC`/`GOARM` for a cross build. CGO stays enabled by default for SQLite.

All generated build files stay inside this checkout's ignored `artifacts/`:

- `artifacts/build/<goos>-<arch>/`: isolated copy of the current working tree, including uncommitted changes, used for Go and Swagger generation.
- `artifacts/publish/<goos>-<arch>/`: application binary, resources, documentation, configuration and empty data/runtime directories.
- `artifacts/cache/go-build/`, `artifacts/cache/go-mod/`: Go caches.
- `artifacts/tmp/`: Go temporary files.

The wrappers scope environment changes to the build process; they do not run `go env -w`. `DOCS=auto` regenerates Swagger when `swag` is on PATH and otherwise uses checked-in documentation. `DOCS=true` requires `swag`; `DOCS=false` always uses existing documentation. Generation and dependency metadata changes happen in the isolated stage, not in tracked source files. The helper uses only the Go standard library.

Build the companion admin frontend separately. Its current output is `rustdesk-api-web/artifacts/publish/web/`; the API helper automatically includes that sibling directory when it contains `index.html`. Set `WEB_DIST` to an explicit frontend output directory to override it. Without one, the API's existing resource files are packaged.

For example:

```sh
GOOS=linux GOARCH=amd64 DOCS=false ./build.sh
WEB_DIST=../rustdesk-api-web/artifacts/publish/web ./build.sh
```

Run the application from the published directory so its relative resource/configuration paths work. Build/publish directories are disposable; keep deployment data outside them.

## CI and Docker

Release workflows use `artifacts/build/source/` for Swagger/Go source staging and `artifacts/build/release/` for archive contents. ZIP/TAR files are written under `artifacts/publish/` and retain the existing internal `./release` directory. Downstream Debian and Docker jobs extract into `artifacts/build/<arch>/release/`; Debian packaging also stays under `artifacts/`.

`Dockerfile` and `Dockerfile_full_s6` consume the extracted release for `BUILDARCH`. Build them from the API repository root after extracting a release archive to `artifacts/build/<arch>/`.

`Dockerfile.dev` and the release workflows default to `Alien21/rustdesk-api-web`, branch `develop-alien21`, output directory `artifacts/publish/web`. Push that frontend branch before running a remote build so the current security fixes are included. Local API builds use the sibling frontend output directly. Docker's `FRONTEND_GIT_REPO`, `FRONTEND_GIT_BRANCH` and `FRONTEND_DIST` arguments allow another checkout; an unmodified upstream frontend uses branch `master` and output `dist`.
