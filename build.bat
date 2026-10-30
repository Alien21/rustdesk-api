@echo off
setlocal
cd /d "%~dp0"
set GOENV=off
set GOTOOLCHAIN=local
set GOWORK=off
set "BUILD_HOST_OS="
set "BUILD_HOST_ARCH="
for /f %%I in ('go env GOHOSTOS') do set "BUILD_HOST_OS=%%I"
for /f %%I in ('go env GOHOSTARCH') do set "BUILD_HOST_ARCH=%%I"
if not defined BUILD_HOST_OS exit /b 2
if not defined BUILD_HOST_ARCH exit /b 2
if not defined GOOS set "GOOS=%BUILD_HOST_OS%"
if not defined GOARCH set "GOARCH=%BUILD_HOST_ARCH%"
if not defined CGO_ENABLED set CGO_ENABLED=1
if not defined DOCS set DOCS=auto
set "BUILD_GOOS=%GOOS%"
set "BUILD_GOARCH=%GOARCH%"
set "BUILD_CGO=%CGO_ENABLED%"
for %%I in ("%CD%\..\artifacts") do set "BUILD_BASE=%%~fI"
if not defined RUSTDESK_ARTIFACTS_DIR goto api_artifacts_root_ready
if not "%RUSTDESK_ARTIFACTS_DIR:~1,2%"==":\" if not "%RUSTDESK_ARTIFACTS_DIR:~1,2%"==":/" if not "%RUSTDESK_ARTIFACTS_DIR:~0,2%"=="\\" (
    echo RUSTDESK_ARTIFACTS_DIR must be an absolute central directory 1>&2
    exit /b 2
)
for %%I in ("%RUSTDESK_ARTIFACTS_DIR%") do set "BUILD_BASE=%%~fI"
:api_artifacts_root_ready
set "BUILD_HOST_PLATFORM=%BUILD_HOST_OS%-%BUILD_HOST_ARCH%"
if "%BUILD_HOST_ARCH%"=="amd64" set "BUILD_HOST_PLATFORM=%BUILD_HOST_OS%-x64"
set "GOCACHE=%BUILD_BASE%\tools\rustdesk-api\go-build"
set "GOMODCACHE=%BUILD_BASE%\tools\rustdesk-api\go-mod"
set "GOPATH=%BUILD_BASE%\tools\rustdesk-api\gopath"
set "GOTMPDIR=%BUILD_BASE%\tmp\%BUILD_HOST_PLATFORM%\rustdesk-api"
set "TMPDIR=%GOTMPDIR%"
set "TMP=%GOTMPDIR%"
set "TEMP=%GOTMPDIR%"
for %%D in ("%GOCACHE%" "%GOMODCACHE%" "%GOTMPDIR%") do if not exist "%%~D" mkdir "%%~D"
set "GOOS=%BUILD_HOST_OS%"
set "GOARCH=%BUILD_HOST_ARCH%"
set CGO_ENABLED=0
go run ./scripts/build/main.go -goos "%BUILD_GOOS%" -goarch "%BUILD_GOARCH%" -cgo "%BUILD_CGO%" -docs "%DOCS%" -web-dist "%WEB_DIST%"
exit /b %errorlevel%
