# Installs or updates the HDMI-CEC host service on Windows 10/11 x64. One command in PowerShell:
#
#   irm https://raw.githubusercontent.com/alexander-litvinovich/entertainment-center/master/hdmi-cec/install.ps1 | iex
#
# The script asks for administrator rights, downloads the newest hdmi-cec release, checks it against
# the release's SHA256SUMS, copies it to %ProgramFiles%\HdmiCecHost, adds that folder to PATH,
# installs (or updates) and starts the HdmiCecHost service, and runs `hdmi-cec setup` when no
# configuration exists yet.

& {
    $ErrorActionPreference = 'Stop'
    # Invoke-WebRequest in Windows PowerShell 5.1 is very slow while drawing its progress bar.
    $ProgressPreference = 'SilentlyContinue'

    $repo = 'alexander-litvinovich/entertainment-center'
    $scriptUrl = "https://raw.githubusercontent.com/$repo/master/hdmi-cec/install.ps1"
    $serviceName = 'HdmiCecHost'
    $installDir = Join-Path $env:ProgramFiles 'HdmiCecHost'
    $exe = Join-Path $installDir 'hdmi-cec.exe'
    $configPath = Join-Path $env:ProgramData 'HdmiCecHost\config.json'

    if (-not [Environment]::Is64BitOperatingSystem) {
        throw 'hdmi-cec needs 64-bit Windows 10 or 11.'
    }

    $principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        Write-Host 'Installing a service needs administrator rights: confirm the UAC prompt.'
        Write-Host 'The installation continues in a new window.'
        if ($PSCommandPath) { $command = "& '$PSCommandPath'" } else { $command = "irm '$scriptUrl' | iex" }
        $shell = (Get-Process -Id $PID).Path
        Start-Process $shell -Verb RunAs -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-NoExit', '-Command', $command)
        return
    }

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    Write-Host 'Looking up the latest hdmi-cec release...'
    # Same choice as the service's updater: the highest published hdmi-cec-vX.Y.Z with the EXE and checksums.
    $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases?per_page=30" -Headers @{ 'User-Agent' = 'hdmi-cec-installer' }
    $release = $releases |
        Where-Object {
            -not $_.draft -and -not $_.prerelease -and $_.tag_name -match '^hdmi-cec-v\d+\.\d+\.\d+$' -and
            ($_.assets.name -contains 'hdmi-cec.exe') -and ($_.assets.name -contains 'SHA256SUMS')
        } |
        Sort-Object { [version]($_.tag_name -replace '^hdmi-cec-v', '') } -Descending |
        Select-Object -First 1
    if (-not $release) {
        throw "No published hdmi-cec release with hdmi-cec.exe in $repo."
    }
    $version = $release.tag_name -replace '^hdmi-cec-v', ''

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('hdmi-cec-' + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading hdmi-cec $version..."
        foreach ($name in 'hdmi-cec.exe', 'SHA256SUMS') {
            $url = ($release.assets | Where-Object { $_.name -eq $name }).browser_download_url
            Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile (Join-Path $tmp $name)
        }
        $expected = $null
        foreach ($line in Get-Content (Join-Path $tmp 'SHA256SUMS')) {
            if ($line -match '^([0-9a-fA-F]{64})\s+\*?hdmi-cec\.exe$') { $expected = $Matches[1] }
        }
        if (-not $expected) {
            throw 'SHA256SUMS has no entry for hdmi-cec.exe.'
        }
        $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp 'hdmi-cec.exe')).Hash
        if ($actual -ne $expected) {
            throw "Checksum mismatch for hdmi-cec.exe: expected $expected, got $actual."
        }

        $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
        if ($service -and $service.Status -ne 'Stopped') {
            Write-Host "Stopping the $serviceName service..."
            Stop-Service -Name $serviceName -Force
            $service.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(30))
        }
        New-Item -ItemType Directory -Force -Path $installDir | Out-Null
        # The stopped service process can hold the EXE for a moment.
        for ($attempt = 1; ; $attempt++) {
            try {
                Copy-Item -Force (Join-Path $tmp 'hdmi-cec.exe') $exe
                break
            } catch {
                if ($attempt -ge 20) { throw }
                Start-Sleep -Milliseconds 500
            }
        }
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }

    Write-Host "Installing the $serviceName service..."
    & $exe install
    if ($LASTEXITCODE -ne 0) {
        if (-not $service) {
            throw "hdmi-cec install failed with exit code $LASTEXITCODE."
        }
        # Releases before 0.8.0 refuse to install over an existing service; it already points here
        # when it was installed by this script.
        Start-Service -Name $serviceName
    }
    (Get-Service -Name $serviceName).WaitForStatus('Running', [TimeSpan]::FromSeconds(30))

    # Add the folder to the machine PATH, keeping it REG_EXPAND_SZ so %SystemRoot% entries still expand.
    $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey('SYSTEM\CurrentControlSet\Control\Session Manager\Environment', $true)
    try {
        $machinePath = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $pathChanged = ($machinePath -split ';') -notcontains $installDir
        if ($pathChanged) {
            $key.SetValue('Path', $machinePath.TrimEnd(';') + ';' + $installDir, [Microsoft.Win32.RegistryValueKind]::ExpandString)
        }
    } finally {
        $key.Close()
    }
    if ($pathChanged) {
        # WM_SETTINGCHANGE lets Explorer hand the new PATH to terminals opened from now on.
        Add-Type -Namespace HdmiCecInstaller -Name Native -MemberDefinition '[DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint msg, UIntPtr wParam, string lParam, uint flags, uint timeout, out UIntPtr result);'
        $ignored = [UIntPtr]::Zero
        [HdmiCecInstaller.Native]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$ignored) | Out-Null
    }
    if (($env:Path -split ';') -notcontains $installDir) {
        $env:Path = $env:Path.TrimEnd(';') + ';' + $installDir
    }

    Write-Host ''
    Write-Host "hdmi-cec $version is installed in $installDir and the $serviceName service is running."
    if (Test-Path $configPath) {
        Write-Host 'Your settings are kept. Change them any time with: hdmi-cec setup'
        return
    }
    Write-Host 'Next: choose the TV input and when the TV turns on and off.'
    Write-Host ''
    & $exe setup
    if ($LASTEXITCODE -ne 0) {
        Write-Host 'Setup did not finish. Run it again any time with: hdmi-cec setup'
    }
}
