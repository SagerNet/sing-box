# Build libbox for Android
# This script sets the required environment variables and builds libbox

Write-Host "Building libbox for Android..." -ForegroundColor Green

# Set Android SDK and NDK paths
$env:ANDROID_HOME = "C:\Users\USERNAME\AppData\Local\Android\Sdk"
$env:ANDROID_SDK_ROOT = "C:\Users\USERNAME\AppData\Local\Android\Sdk"
$env:ANDROID_NDK_HOME = "C:\Users\USERNAME\AppData\Local\Android\Sdk\ndk\27.0.12077973"
$env:JAVA_HOME="C:\openjdk17\jdk-17.0.13+11"

Write-Host "ANDROID_HOME: $env:ANDROID_HOME" -ForegroundColor Cyan
Write-Host "ANDROID_NDK_HOME: $env:ANDROID_NDK_HOME" -ForegroundColor Cyan

# Build libbox
Write-Host "`nStarting build..." -ForegroundColor Yellow
make lib_android

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nBuild completed successfully!" -ForegroundColor Green
    Write-Host "`nGenerated AAR location:" -ForegroundColor Cyan
    Write-Host "../sing-box-for-android/app/libs/libbox.aar" -ForegroundColor White
    
    # Check if file exists
    if (Test-Path "../sing-box-for-android/app/libs/libbox.aar") {
        $fileSize = (Get-Item "../sing-box-for-android/app/libs/libbox.aar").Length / 1MB
        Write-Host "File size: $([math]::Round($fileSize, 2)) MB" -ForegroundColor White
    }
} else {
    Write-Host "`nBuild failed with error code $LASTEXITCODE" -ForegroundColor Red
}

