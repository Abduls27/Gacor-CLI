$ErrorActionPreference = "Stop"

$repo          = "Abduls27/Gacor-CLI"   # change to your owner/repo
$installDir    = $env:GACOR_HOME
if ([string]::IsNullOrWhiteSpace($installDir)) {
    $installDir = Join-Path $env:USERPROFILE ".gacor"
}
$exePath       = "$installDir\gacor.exe"
$uninstallPath = "$installDir\uninstall.ps1"
$regPath       = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\gacor"

# 0. Get the latest release info from GitHub
Write-Host "-> Checking latest version..." -ForegroundColor Cyan
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
} catch {
    Write-Host "x Failed to fetch release information. Check your internet connection or repository name." -ForegroundColor Red
    exit 1
}

$appVersion = $release.tag_name.TrimStart("v")
$asset = $release.assets | Where-Object { $_.name -eq "gacor.exe" }

if (-not $asset) {
    Write-Host "x The gacor.exe binary was not found in release $($release.tag_name)." -ForegroundColor Red
    exit 1
}

$downloadUrl = $asset.browser_download_url

# 1. Detect whether this is a fresh install or an update
$existing = Get-ItemProperty -Path $regPath -ErrorAction SilentlyContinue
$isUpdate = $null -ne $existing
$oldVersion = if ($isUpdate) { $existing.DisplayVersion } else { $null }

if ($isUpdate -and $oldVersion -eq $appVersion) {
    Write-Host "* gacor $appVersion is already up to date. Nothing to update." -ForegroundColor Yellow
    Write-Host "  Run again with -Force if you still want to reinstall." -ForegroundColor Gray
    if (-not $env:GACOR_FORCE) {
        exit 0
    }
} elseif ($isUpdate) {
    Write-Host "* Updating gacor $oldVersion -> $appVersion ..." -ForegroundColor Green
} else {
    Write-Host "* Installing gacor $appVersion..." -ForegroundColor Green
}

# 2. VC Redist
$vc = Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\x64" -ErrorAction SilentlyContinue
if (-not $vc -or $vc.Installed -ne 1) {
    Write-Host "-> Installing Visual C++ Redistributable..." -ForegroundColor Cyan
    $vcPath = "$env:TEMP\vc_redist.x64.exe"
    Invoke-WebRequest "https://aka.ms/vs/17/release/vc_redist.x64.exe" -OutFile $vcPath
    Start-Process $vcPath -ArgumentList "/install","/quiet","/norestart" -Wait -Verb RunAs
}

# 3. Stop running processes so the exe is not locked during overwrite
if ($isUpdate) {
    Write-Host "-> Stopping running services..." -ForegroundColor Cyan
    Get-Process php-cgi, nginx, gacor -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500
}

# 4. Download the binary from the GitHub Release
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Write-Host "-> Downloading gacor.exe ($appVersion)..." -ForegroundColor Cyan
$tmpExe = "$exePath.download"
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpExe
    Move-Item -Path $tmpExe -Destination $exePath -Force
} catch {
    Write-Host "x Failed to download/overwrite gacor.exe - the file may still be in use by another process." -ForegroundColor Red
    Write-Host "  Close all terminals running gacor, then try again." -ForegroundColor Gray
    if (Test-Path $tmpExe) { Remove-Item $tmpExe -Force -ErrorAction SilentlyContinue }
    exit 1
}

# 5. PATH
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
$userPathEntries = @($userPath -split ";" | Where-Object { $_ })
if ($userPathEntries -notcontains $installDir) {
    if ([string]::IsNullOrWhiteSpace($userPath)) {
        $newUserPath = $installDir
    } else {
        $newUserPath = "$userPath;$installDir"
    }
    [Environment]::SetEnvironmentVariable("PATH", $newUserPath, "User")
}

# 6. Write uninstall.ps1 to the install directory
$uninstallScript = @'
$ErrorActionPreference = "Stop"

$installDir = $env:GACOR_HOME
if ([string]::IsNullOrWhiteSpace($installDir)) {
    $installDir = Join-Path $env:USERPROFILE ".gacor"
}

Write-Host "Uninstalling gacor..." -ForegroundColor Yellow

Get-Process php-cgi, nginx, gacor -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
$entries  = @($userPath -split ";" | Where-Object { $_ -and $_ -ne $installDir })
[Environment]::SetEnvironmentVariable("PATH", ($entries -join ";"), "User")

Remove-Item -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\gacor" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path $installDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "gacor has been removed." -ForegroundColor Green
'@

Set-Content -Path $uninstallPath -Value $uninstallScript -Encoding UTF8

# 7. Register / update registry entries in Apps & Features
New-Item -Path $regPath -Force | Out-Null
Set-ItemProperty -Path $regPath -Name "DisplayName"     -Value "Gacor"
Set-ItemProperty -Path $regPath -Name "DisplayVersion"  -Value $appVersion
Set-ItemProperty -Path $regPath -Name "Publisher"       -Value "Abduls27"
Set-ItemProperty -Path $regPath -Name "InstallLocation" -Value $installDir
Set-ItemProperty -Path $regPath -Name "DisplayIcon"     -Value $exePath
Set-ItemProperty -Path $regPath -Name "UninstallString" -Value "powershell.exe -ExecutionPolicy Bypass -File `"$uninstallPath`""
Set-ItemProperty -Path $regPath -Name "NoModify"        -Value 1 -Type DWord
Set-ItemProperty -Path $regPath -Name "NoRepair"        -Value 1 -Type DWord
Set-ItemProperty -Path $regPath -Name "EstimatedSize"   -Value 20000 -Type DWord

Write-Host ""
if ($isUpdate) {
    Write-Host ('OK gacor was successfully updated to ' + $appVersion) -ForegroundColor Green
} else {
    Write-Host ('OK gacor ' + $appVersion + ' is installed in ' + $installDir) -ForegroundColor Green
}
Write-Host "  Open a new terminal, then run: gacor" -ForegroundColor Gray
Write-Host "  To uninstall, search for 'gacor' in Settings > Apps, or run uninstall.ps1" -ForegroundColor Gray