@echo off
setlocal EnableExtensions DisableDelayedExpansion

rem Use the same image and dependency volume for Windows folders and WSL shares.
set "GO_IMAGE=golang:1.27.3"
set "PROJECT_DIR=%~dp0"
set "BUILD_MODE=%~1"
if not "%~2"=="" goto usage
if not "%BUILD_MODE%"=="" if /I not "%BUILD_MODE%"=="test" goto usage

rem A WSL UNC share cannot be passed to Docker as a Windows bind source.
if /I "%PROJECT_DIR:~0,16%"=="\\wsl.localhost\" goto wsl_path
if /I "%PROJECT_DIR:~0,7%"=="\\wsl$\" goto wsl_path
if "%PROJECT_DIR:~0,2%"=="\\" (
  echo ERROR: Copy the project to a local drive or use a WSL share.
  exit /b 1
)

where docker >nul 2>&1
if errorlevel 1 (
  echo ERROR: Docker Desktop is required to build. Enable Linux containers.
  exit /b 1
)
docker info --format={{.OSType}} 2>nul | findstr /l /c:linux >nul
if errorlevel 1 goto docker_unavailable
if /I "%BUILD_MODE%"=="test" goto windows_test
if not exist "%PROJECT_DIR%bin\" mkdir "%PROJECT_DIR%bin"
if errorlevel 1 exit /b 1
docker run --rm --mount "type=bind,source=%PROJECT_DIR%.,target=/src" -w /src -v wslproxy-go-mod:/go/pkg/mod -v wslproxy-go-build:/root/.cache/go-build -e GOPROXY=off -e GOTOOLCHAIN=local -e CGO_ENABLED=0 -e GOOS=windows -e GOARCH=amd64 %GO_IMAGE% go build -mod=readonly -trimpath -o bin/proxy.exe ./cmd/proxy
if errorlevel 1 goto build_failed
goto success

:windows_test
docker run --rm --mount "type=bind,source=%PROJECT_DIR%.,target=/src" -w /src -v wslproxy-go-mod:/go/pkg/mod -v wslproxy-go-build:/root/.cache/go-build -e GOPROXY=off -e GOTOOLCHAIN=local %GO_IMAGE% sh -eu -c "go test -mod=readonly -race ./... && go vet -mod=readonly ./..."
exit /b %errorlevel%

:wsl_path
where wsl.exe >nul 2>&1
if errorlevel 1 (
  echo ERROR: wsl.exe is required to build a project stored in WSL.
  exit /b 1
)
for /f "tokens=1,2,* delims=\" %%A in ("%PROJECT_DIR%") do (
  set "WSL_DISTRO=%%B"
  set "WSL_RELATIVE=%%C"
)
set "WSL_PROJECT=/%WSL_RELATIVE:\=/%"
rem WSL distribution names must be passed without quotes in CMD.
rem Validate before expanding the name as an unquoted argument.
set WSL_DISTRO| findstr /r /x "WSL_DISTRO=[A-Za-z0-9][A-Za-z0-9._-]*" >nul
if errorlevel 1 (
  echo ERROR: Unsupported WSL distribution name in the project path.
  exit /b 1
)
rem WSL emits LF line endings; FINDSTR /X expects Windows line endings.
wsl.exe -d %WSL_DISTRO% --exec docker info --format={{.OSType}} | findstr /l /c:linux >nul
if errorlevel 1 goto docker_unavailable
if /I "%BUILD_MODE%"=="test" goto wsl_test
wsl.exe -d %WSL_DISTRO% --exec mkdir -p "%WSL_PROJECT%bin"
if errorlevel 1 exit /b 1
wsl.exe -d %WSL_DISTRO% --exec docker run --rm --mount "type=bind,source=%WSL_PROJECT%,target=/src" -w /src -v wslproxy-go-mod:/go/pkg/mod -v wslproxy-go-build:/root/.cache/go-build -e GOPROXY=off -e GOTOOLCHAIN=local -e CGO_ENABLED=0 -e GOOS=windows -e GOARCH=amd64 %GO_IMAGE% go build -mod=readonly -trimpath -o bin/proxy.exe ./cmd/proxy
if errorlevel 1 goto build_failed
goto success

:wsl_test
wsl.exe -d %WSL_DISTRO% --exec docker run --rm --mount "type=bind,source=%WSL_PROJECT%,target=/src" -w /src -v wslproxy-go-mod:/go/pkg/mod -v wslproxy-go-build:/root/.cache/go-build -e GOPROXY=off -e GOTOOLCHAIN=local %GO_IMAGE% sh -eu -c "go test -mod=readonly -race ./... && go vet -mod=readonly ./..."
exit /b %errorlevel%

:docker_unavailable
echo ERROR: Docker Linux engine is unavailable. Start Docker Desktop.
echo For a WSL project, also enable Docker Desktop integration for its distribution.
exit /b 1

:build_failed
echo ERROR: Build failed. Check the output above.
echo Dependencies must be downloaded first; see README.md.
exit /b 1

:success
echo Built: "%PROJECT_DIR%bin\proxy.exe"
exit /b 0

:usage
echo Usage: build.cmd [test]
exit /b 2
