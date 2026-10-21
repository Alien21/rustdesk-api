@echo off
setlocal
cd /d "%~dp0"
if not defined GOOS for /f %%I in ('go env GOHOSTOS') do set GOOS=%%I
if not defined GOARCH for /f %%I in ('go env GOHOSTARCH') do set GOARCH=%%I
if not defined CGO_ENABLED set CGO_ENABLED=1
if not defined DOCS set DOCS=auto
set "BUILD_GOOS=%GOOS%"
set "BUILD_GOARCH=%GOARCH%"
set "BUILD_CGO=%CGO_ENABLED%"
set "GOCACHE=%CD%\artifacts\cache\go-build"
set "GOMODCACHE=%CD%\artifacts\cache\go-mod"
set "GOTMPDIR=%CD%\artifacts\tmp"
set "TMPDIR=%GOTMPDIR%"
set "TMP=%GOTMPDIR%"
set "TEMP=%GOTMPDIR%"
for %%D in ("%GOCACHE%" "%GOMODCACHE%" "%GOTMPDIR%") do if not exist "%%~D" mkdir "%%~D"
for /f %%I in ('go env GOHOSTOS') do set GOOS=%%I
for /f %%I in ('go env GOHOSTARCH') do set GOARCH=%%I
set CGO_ENABLED=0
go run ./scripts/build/main.go -goos "%BUILD_GOOS%" -goarch "%BUILD_GOARCH%" -cgo "%BUILD_CGO%" -docs "%DOCS%" -web-dist "%WEB_DIST%"
exit /b %errorlevel%
