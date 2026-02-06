@echo off
REM Windows Build Script for Distributed Proxy System
REM Run this to build all client applications

echo ========================================
echo  Distributed Proxy Client Builder
echo ========================================
echo.

REM Check if Go is installed
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go is not installed or not in PATH
    echo Please install Go from: https://go.dev/dl/
    pause
    exit /b 1
)

echo [INFO] Go version:
go version
echo.

REM Create build directory
if not exist "build" mkdir build
if not exist "build\config" mkdir build\config

echo ========================================
echo Step 1: Installing Dependencies
echo ========================================
echo.

REM Install dependencies
echo Installing YAML parser...
go get gopkg.in/yaml.v3

echo.
echo ========================================
echo Step 2: Building CLI Client
echo ========================================
echo.

cd client-cli
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] client-cli directory not found
    pause
    exit /b 1
)

echo Building proxy-cli.exe...
go build -ldflags="-s -w" -o ..\build\proxy-cli.exe main.go

if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to build CLI client
    cd ..
    pause
    exit /b 1
)

echo [SUCCESS] CLI client built: build\proxy-cli.exe
cd ..

echo.
echo ========================================
echo Step 3: Building GUI Client (Optional)
echo ========================================
echo.

set /p BUILD_GUI="Build GUI client? (requires GCC) [y/N]: "
if /i "%BUILD_GUI%"=="y" (
    echo Installing Fyne UI library...
    go get fyne.io/fyne/v2
    go get fyne.io/fyne/v2/app
    go get fyne.io/fyne/v2/container
    go get fyne.io/fyne/v2/widget
    
    cd client-gui
    echo Building proxy-gui.exe...
    go build -ldflags="-s -w -H=windowsgui" -o ..\build\proxy-gui.exe main.go
    
    if %ERRORLEVEL% NEQ 0 (
        echo [WARNING] Failed to build GUI client (GCC might be missing)
        echo Continuing with CLI client only...
    ) else (
        echo [SUCCESS] GUI client built: build\proxy-gui.exe
    )
    cd ..
) else (
    echo [INFO] Skipping GUI client build
)

echo.
echo ========================================
echo Step 4: Copying Configuration Files
echo ========================================
echo.

copy config\client.yaml build\config\ >nul
echo [SUCCESS] Copied config\client.yaml

echo.
echo ========================================
echo Step 5: Copying Documentation
echo ========================================
echo.

copy README.md build\ >nul
copy WINDOWS_GUIDE.md build\ >nul
copy RELAY_GATEWAY_EXPLAINED.md build\ >nul
copy ARCHITECTURE_EXPLAINED.md build\ >nul
echo [SUCCESS] Documentation copied

echo.
echo ========================================
echo Step 6: Creating README for Build
echo ========================================
echo.

(
echo # Distributed Proxy Client - Windows Build
echo.
echo ## Files Included
echo.
echo - proxy-cli.exe - Command-line interface
echo - proxy-gui.exe - Graphical interface ^(if built^)
echo - config/client.yaml - Configuration file
echo - README.md - Full documentation
echo - WINDOWS_GUIDE.md - Windows-specific guide
echo.
echo ## Quick Start
echo.
echo 1. Edit config/client.yaml with your server addresses
echo 2. Run: proxy-cli.exe -url http://example.com
echo 3. Or double-click: proxy-gui.exe
echo.
echo ## Configuration
echo.
echo Edit config/client.yaml:
echo - Set upstream_servers to your server IPs
echo - Adjust chunk_size if needed
echo - Configure encryption settings
echo.
echo ## Usage Examples
echo.
echo GET request:
echo   proxy-cli.exe -url http://example.com
echo.
echo POST request:
echo   proxy-cli.exe -method POST -url http://api.example.com -data "{\"test\":\"data\"}"
echo.
echo Interactive mode:
echo   proxy-cli.exe -i
echo.
echo For full documentation, see WINDOWS_GUIDE.md
) > build\README.txt

echo [SUCCESS] Created build\README.txt

echo.
echo ========================================
echo Step 7: Creating Archive
echo ========================================
echo.

REM Create ZIP archive using PowerShell
powershell -Command "Compress-Archive -Path build\* -DestinationPath proxy-client-windows.zip -Force"

if %ERRORLEVEL% NEQ 0 (
    echo [WARNING] Failed to create ZIP archive
    echo You can manually zip the build\ folder
) else (
    echo [SUCCESS] Created: proxy-client-windows.zip
)

echo.
echo ========================================
echo Build Complete!
echo ========================================
echo.
echo Output files:
echo   - build\proxy-cli.exe
if exist "build\proxy-gui.exe" echo   - build\proxy-gui.exe
echo   - build\config\client.yaml
echo   - proxy-client-windows.zip
echo.
echo Next steps:
echo   1. Extract proxy-client-windows.zip
echo   2. Edit config\client.yaml
echo   3. Run proxy-cli.exe or proxy-gui.exe
echo.
echo See WINDOWS_GUIDE.md for detailed instructions
echo.

pause
