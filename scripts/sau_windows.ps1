param(
    [ValidateSet("run", "start", "stop", "restart", "status", "install-task", "uninstall-task")]
    [string]$Action = "run",
    [string]$EnvFile = "",
    [string]$TaskName = "OmniBull SAU"
)

$ErrorActionPreference = "Stop"

$ScriptPath = $MyInvocation.MyCommand.Path
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = [System.IO.Path]::GetFullPath((Join-Path $ScriptDir ".."))

if ([string]::IsNullOrWhiteSpace($EnvFile)) {
    $EnvFile = Join-Path $RootDir "deploy\env\omnibull.windows.env"
}

function Import-EnvFile {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $text = [string]$line
        if ([string]::IsNullOrWhiteSpace($text)) {
            continue
        }
        if ($text.TrimStart().StartsWith("#")) {
            continue
        }
        $parts = $text.Split("=", 2)
        if ($parts.Count -ne 2) {
            continue
        }
        $name = $parts[0].Trim()
        $value = $parts[1].Trim()
        if ([string]::IsNullOrWhiteSpace($name)) {
            continue
        }
        if ($value.Length -ge 2) {
            if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
        }
        [System.Environment]::SetEnvironmentVariable($name, $value, "Process")
    }
}

Import-EnvFile -Path $EnvFile

$RuntimeDir = if ($env:SAU_RUNTIME_DIR) { $env:SAU_RUNTIME_DIR } else { Join-Path $RootDir "runtime" }
$LogDir = if ($env:SAU_LOG_DIR) { $env:SAU_LOG_DIR } else { Join-Path $RootDir "logs" }

$BackendPidFile = Join-Path $RuntimeDir "sau_backend.pid"
$FrontendPidFile = Join-Path $RuntimeDir "sau_frontend.pid"
$LauncherPidFile = Join-Path $RuntimeDir "sau_stack.pid"

$BackendLogFile = Join-Path $LogDir "sau_backend.start.log"
$BackendErrorLogFile = Join-Path $LogDir "sau_backend.start.err.log"
$FrontendLogFile = Join-Path $LogDir "sau_frontend.start.log"
$FrontendErrorLogFile = Join-Path $LogDir "sau_frontend.start.err.log"
$LauncherLogFile = Join-Path $LogDir "sau_stack.log"
$LauncherConsoleLogFile = Join-Path $LogDir "sau_stack.console.log"
$LauncherErrorLogFile = Join-Path $LogDir "sau_stack.err.log"

$BackendPort = if ($env:SAU_BACKEND_PORT) { [int]$env:SAU_BACKEND_PORT } else { 5409 }
$FrontendPort = if ($env:SAU_FRONTEND_PORT) { [int]$env:SAU_FRONTEND_PORT } else { 5173 }
$FrontendHost = if ($env:SAU_FRONTEND_HOST) { $env:SAU_FRONTEND_HOST } else { "0.0.0.0" }
$FrontendMode = if ($env:SAU_FRONTEND_MODE) { $env:SAU_FRONTEND_MODE } else { "preview" }
$FrontendBuildOnStart = if ($env:SAU_FRONTEND_BUILD_ON_START) { $env:SAU_FRONTEND_BUILD_ON_START } else { "auto" }
$FrontendInstallOnStart = if ($env:SAU_FRONTEND_INSTALL_ON_START) { $env:SAU_FRONTEND_INSTALL_ON_START } else { "1" }

$FrontendDir = Join-Path $RootDir "sau_frontend"
$FrontendDistFile = Join-Path $FrontendDir "dist\index.html"

New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogDir -Force | Out-Null

function Write-Log {
    param(
        [string]$Level,
        [string]$Message
    )

    $line = "[{0}] [{1}] {2}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Level, $Message
    $line | Tee-Object -FilePath $LauncherLogFile -Append
}

function Get-ListeningPid {
    param([int]$Port)

    $connection = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue |
        Sort-Object OwningProcess |
        Select-Object -First 1
    if ($null -eq $connection) {
        return $null
    }

    return [int]$connection.OwningProcess
}

function Wait-ForListeningPid {
    param(
        [int]$Port,
        [int]$TimeoutSeconds = 60
    )

    for ($i = 0; $i -lt $TimeoutSeconds; $i += 1) {
        $listeningPid = Get-ListeningPid -Port $Port
        if ($listeningPid) {
            return $listeningPid
        }
        Start-Sleep -Seconds 1
    }

    return $null
}

function Get-StoredPid {
    param([string]$PidFile)

    if (-not (Test-Path -LiteralPath $PidFile)) {
        return $null
    }

    $value = (Get-Content -LiteralPath $PidFile -Raw).Trim()
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $null
    }

    return [int]$value
}

function Test-ProcessRunning {
    param([int]$TargetPid)

    if (-not $TargetPid) {
        return $false
    }

    try {
        Get-Process -Id $TargetPid -ErrorAction Stop | Out-Null
        return $true
    }
    catch {
        return $false
    }
}

function Remove-StalePidFile {
    param([string]$PidFile)

    $targetPid = Get-StoredPid -PidFile $PidFile
    if ($targetPid -and -not (Test-ProcessRunning -TargetPid $targetPid)) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
    }
}

function Stop-ProcessFromFile {
    param(
        [string]$Name,
        [string]$PidFile
    )

    $targetPid = Get-StoredPid -PidFile $PidFile
    if (-not $targetPid) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
        return
    }
    if (-not (Test-ProcessRunning -TargetPid $targetPid)) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
        return
    }

    Write-Log -Level "INFO" -Message ("Stopping {0} pid={1}" -f $Name, $targetPid)
    try {
        Stop-Process -Id $targetPid -ErrorAction Stop
    }
    catch {
    }

    $stopped = $false
    for ($i = 0; $i -lt 20; $i += 1) {
        Start-Sleep -Seconds 1
        if (-not (Test-ProcessRunning -TargetPid $targetPid)) {
            $stopped = $true
            break
        }
    }

    if (-not $stopped -and (Test-ProcessRunning -TargetPid $targetPid)) {
        Write-Log -Level "WARNING" -Message ("{0} did not exit in time, force killing pid={1}" -f $Name, $targetPid)
        try {
            Stop-Process -Id $targetPid -Force -ErrorAction Stop
        }
        catch {
        }
    }

    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
}

function Stop-ProcessByPort {
    param(
        [string]$Name,
        [int]$Port,
        [string]$PidFile = ""
    )

    $listeningPid = Get-ListeningPid -Port $Port
    if (-not $listeningPid) {
        return
    }

    Write-Log -Level "INFO" -Message ("Stopping {0} on port {1} pid={2}" -f $Name, $Port, $listeningPid)
    try {
        Stop-Process -Id $listeningPid -Force -ErrorAction Stop
    }
    catch {
    }

    if (-not [string]::IsNullOrWhiteSpace($PidFile)) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
    }
}

function Resolve-PythonSpec {
    $candidates = @()
    if ($env:SAU_PYTHON_BIN) {
        $candidates += @{ FilePath = $env:SAU_PYTHON_BIN; Prefix = @() }
    }
    $candidates += @{ FilePath = (Join-Path $RootDir ".venv\Scripts\python.exe"); Prefix = @() }
    $candidates += @{ FilePath = (Join-Path $RootDir "venv\Scripts\python.exe"); Prefix = @() }
    $candidates += @{ FilePath = (Join-Path $RootDir "env\Scripts\python.exe"); Prefix = @() }

    foreach ($candidate in $candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate.FilePath)) {
            continue
        }
        if (Test-Path -LiteralPath $candidate.FilePath) {
            return $candidate
        }
    }

    foreach ($commandName in @("python.exe", "python")) {
        $command = Get-Command $commandName -ErrorAction SilentlyContinue
        if ($null -ne $command) {
            return @{ FilePath = $command.Source; Prefix = @() }
        }
    }

    $pyCommand = Get-Command "py.exe" -ErrorAction SilentlyContinue
    if ($null -ne $pyCommand) {
        return @{ FilePath = $pyCommand.Source; Prefix = @("-3") }
    }

    throw "Python runtime not found. Set SAU_PYTHON_BIN or create .venv."
}

function Resolve-NpmPath {
    $candidates = @()
    if ($env:SAU_NPM_BIN) {
        $candidates += $env:SAU_NPM_BIN
    }
    $candidates += @(
        "$env:ProgramFiles\nodejs\npm.cmd",
        "${env:ProgramFiles(x86)}\nodejs\npm.cmd"
    )

    foreach ($candidate in $candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }
        if (Test-Path -LiteralPath $candidate) {
            return $candidate
        }
    }

    foreach ($commandName in @("npm.cmd", "npm")) {
        $command = Get-Command $commandName -ErrorAction SilentlyContinue
        if ($null -ne $command) {
            return $command.Source
        }
    }

    throw "npm not found. Set SAU_NPM_BIN or install Node.js."
}

function Ensure-ExecutableDirOnPath {
    param([string]$ExecutablePath)

    if ([string]::IsNullOrWhiteSpace($ExecutablePath)) {
        return
    }

    $directory = Split-Path -Parent $ExecutablePath
    if ([string]::IsNullOrWhiteSpace($directory)) {
        return
    }

    $segments = @($env:Path -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($segments -contains $directory) {
        return
    }

    $env:Path = "{0};{1}" -f $directory, ($segments -join ';')
}

function Invoke-LoggedCommand {
    param(
        [string]$FilePath,
        [string[]]$Arguments,
        [string]$WorkingDirectory,
        [string]$LogFile
    )

    Push-Location $WorkingDirectory
    try {
        & $FilePath @Arguments *>> $LogFile
        if ($LASTEXITCODE -ne 0) {
            throw ("Command failed with exit code {0}: {1} {2}" -f $LASTEXITCODE, $FilePath, ($Arguments -join " "))
        }
    }
    finally {
        Pop-Location
    }
}

function Test-FrontendSourcesNewerThanDist {
    if (-not (Test-Path -LiteralPath $FrontendDistFile)) {
        return $true
    }

    $distTime = (Get-Item -LiteralPath $FrontendDistFile).LastWriteTimeUtc
    foreach ($candidate in @(
        (Join-Path $FrontendDir "package.json"),
        (Join-Path $FrontendDir "package-lock.json")
    )) {
        if ((Test-Path -LiteralPath $candidate) -and ((Get-Item -LiteralPath $candidate).LastWriteTimeUtc -gt $distTime)) {
            return $true
        }
    }

    foreach ($folder in @("src", "public")) {
        $fullPath = Join-Path $FrontendDir $folder
        if (-not (Test-Path -LiteralPath $fullPath)) {
            continue
        }
        $newerItem = Get-ChildItem -LiteralPath $fullPath -Recurse -File | Where-Object { $_.LastWriteTimeUtc -gt $distTime } | Select-Object -First 1
        if ($null -ne $newerItem) {
            return $true
        }
    }

    return $false
}

function Install-FrontendDependenciesIfNeeded {
    param([string]$NpmPath)

    if ($FrontendInstallOnStart -ne "1") {
        return
    }
    if (Test-Path -LiteralPath (Join-Path $FrontendDir "node_modules")) {
        return
    }

    Write-Log -Level "INFO" -Message "Installing sau_frontend dependencies"
    if (Test-Path -LiteralPath (Join-Path $FrontendDir "package-lock.json")) {
        Invoke-LoggedCommand -FilePath $NpmPath -Arguments @("ci") -WorkingDirectory $FrontendDir -LogFile $FrontendLogFile
        return
    }
    Invoke-LoggedCommand -FilePath $NpmPath -Arguments @("install") -WorkingDirectory $FrontendDir -LogFile $FrontendLogFile
}

function Build-FrontendIfNeeded {
    param([string]$NpmPath)

    $shouldBuild = $false
    switch ($FrontendBuildOnStart.ToLowerInvariant()) {
        "always" { $shouldBuild = $true }
        "1" { $shouldBuild = $true }
        "never" {
            if (-not (Test-Path -LiteralPath $FrontendDistFile)) {
                throw "sau_frontend build output not found and SAU_FRONTEND_BUILD_ON_START=never"
            }
            $shouldBuild = $false
        }
        "0" {
            if (-not (Test-Path -LiteralPath $FrontendDistFile)) {
                throw "sau_frontend build output not found and SAU_FRONTEND_BUILD_ON_START=0"
            }
            $shouldBuild = $false
        }
        default { $shouldBuild = Test-FrontendSourcesNewerThanDist }
    }

    if (-not $shouldBuild) {
        return
    }

    Write-Log -Level "INFO" -Message ("Building sau_frontend for {0} mode" -f $FrontendMode)
    Invoke-LoggedCommand -FilePath $NpmPath -Arguments @("run", "build") -WorkingDirectory $FrontendDir -LogFile $FrontendLogFile
}

function Start-Backend {
    param($PythonSpec)

    Remove-StalePidFile -PidFile $BackendPidFile
    $backendPid = Get-StoredPid -PidFile $BackendPidFile
    $listeningPid = Get-ListeningPid -Port $BackendPort
    if ($listeningPid -and (Test-ProcessRunning -TargetPid $listeningPid)) {
        Set-Content -LiteralPath $BackendPidFile -Value $listeningPid -Encoding ASCII
        Write-Log -Level "INFO" -Message ("SAU backend already running pid={0} port={1}" -f $listeningPid, $BackendPort)
        return
    }
    if ($backendPid -and (Test-ProcessRunning -TargetPid $backendPid)) {
        Write-Log -Level "INFO" -Message ("SAU backend already running pid={0}" -f $backendPid)
        return
    }

    Write-Log -Level "INFO" -Message ("Starting SAU backend on port {0}" -f $BackendPort)
    $env:PYTHONUNBUFFERED = "1"
    $env:SAU_BACKEND_PORT = [string]$BackendPort
    $args = @()
    $args += $PythonSpec.Prefix
    $args += (Join-Path $RootDir "sau_backend.py")

    $process = Start-Process -FilePath $PythonSpec.FilePath `
        -ArgumentList $args `
        -WorkingDirectory $RootDir `
        -RedirectStandardOutput $BackendLogFile `
        -RedirectStandardError $BackendErrorLogFile `
        -WindowStyle Hidden `
        -PassThru

    $listeningPid = Wait-ForListeningPid -Port $BackendPort -TimeoutSeconds 30
    if ($listeningPid) {
        Set-Content -LiteralPath $BackendPidFile -Value $listeningPid -Encoding ASCII
        return
    }

    if (Test-ProcessRunning -TargetPid $process.Id) {
        Set-Content -LiteralPath $BackendPidFile -Value $process.Id -Encoding ASCII
    }

    throw ("SAU backend failed to bind port {0}. Check {1} and {2}" -f $BackendPort, $BackendLogFile, $BackendErrorLogFile)
}

function Start-Frontend {
    param([string]$NpmPath)

    Remove-StalePidFile -PidFile $FrontendPidFile
    $frontendPid = Get-StoredPid -PidFile $FrontendPidFile
    $listeningPid = Get-ListeningPid -Port $FrontendPort
    if ($listeningPid -and (Test-ProcessRunning -TargetPid $listeningPid)) {
        Set-Content -LiteralPath $FrontendPidFile -Value $listeningPid -Encoding ASCII
        Write-Log -Level "INFO" -Message ("SAU frontend already running pid={0} port={1}" -f $listeningPid, $FrontendPort)
        return
    }
    if ($frontendPid -and (Test-ProcessRunning -TargetPid $frontendPid)) {
        Write-Log -Level "INFO" -Message ("SAU frontend already running pid={0}" -f $frontendPid)
        return
    }

    Install-FrontendDependenciesIfNeeded -NpmPath $NpmPath
    if ($FrontendMode -eq "preview") {
        Build-FrontendIfNeeded -NpmPath $NpmPath
    }

    Write-Log -Level "INFO" -Message ("Starting SAU frontend in {0} mode on {1}:{2}" -f $FrontendMode, $FrontendHost, $FrontendPort)

    $args = @("run")
    if ($FrontendMode -eq "preview") {
        $args += @("preview", "--", "--host", $FrontendHost, "--port", [string]$FrontendPort, "--strictPort")
    }
    else {
        $args += @("dev", "--", "--host", $FrontendHost, "--port", [string]$FrontendPort, "--strictPort")
    }

    $process = Start-Process -FilePath $NpmPath `
        -ArgumentList $args `
        -WorkingDirectory $FrontendDir `
        -RedirectStandardOutput $FrontendLogFile `
        -RedirectStandardError $FrontendErrorLogFile `
        -WindowStyle Hidden `
        -PassThru

    $listeningPid = Wait-ForListeningPid -Port $FrontendPort -TimeoutSeconds 30
    if ($listeningPid) {
        Set-Content -LiteralPath $FrontendPidFile -Value $listeningPid -Encoding ASCII
        return
    }

    if (Test-ProcessRunning -TargetPid $process.Id) {
        Set-Content -LiteralPath $FrontendPidFile -Value $process.Id -Encoding ASCII
    }

    throw ("SAU frontend failed to bind port {0}. Check {1} and {2}" -f $FrontendPort, $FrontendLogFile, $FrontendErrorLogFile)
}

function Start-RunLoop {
    $pythonSpec = Resolve-PythonSpec
    $npmPath = Resolve-NpmPath
    Ensure-ExecutableDirOnPath -ExecutablePath $pythonSpec.FilePath
    Ensure-ExecutableDirOnPath -ExecutablePath $npmPath

    if (-not (Test-Path -LiteralPath (Join-Path $RootDir "conf.py"))) {
        throw ("conf.py not found at {0}" -f (Join-Path $RootDir "conf.py"))
    }
    if (-not (Test-Path -LiteralPath $FrontendDir)) {
        throw ("sau_frontend directory not found at {0}" -f $FrontendDir)
    }

    Remove-StalePidFile -PidFile $LauncherPidFile
    Set-Content -LiteralPath $LauncherPidFile -Value $PID -Encoding ASCII

    Start-Backend -PythonSpec $pythonSpec
    Start-Frontend -NpmPath $npmPath

    Write-Log -Level "INFO" -Message ("SAU stack started launcher_pid={0} backend_pid={1} frontend_pid={2}" -f $PID, (Get-StoredPid -PidFile $BackendPidFile), (Get-StoredPid -PidFile $FrontendPidFile))

    try {
        while ($true) {
            $backendPid = Get-StoredPid -PidFile $BackendPidFile
            $frontendPid = Get-StoredPid -PidFile $FrontendPidFile
            if (-not (Test-ProcessRunning -TargetPid $backendPid)) {
                throw ("SAU backend exited unexpectedly pid={0}" -f $backendPid)
            }
            if (-not (Test-ProcessRunning -TargetPid $frontendPid)) {
                throw ("SAU frontend exited unexpectedly pid={0}" -f $frontendPid)
            }
            Start-Sleep -Seconds 2
        }
    }
    finally {
        Write-Log -Level "INFO" -Message "Stopping SAU stack"
        Stop-ProcessFromFile -Name "SAU frontend" -PidFile $FrontendPidFile
        Stop-ProcessFromFile -Name "SAU backend" -PidFile $BackendPidFile
        Remove-Item -LiteralPath $LauncherPidFile -Force -ErrorAction SilentlyContinue
    }
}

function Start-Daemon {
    $pythonSpec = Resolve-PythonSpec
    $npmPath = Resolve-NpmPath
    Ensure-ExecutableDirOnPath -ExecutablePath $pythonSpec.FilePath
    Ensure-ExecutableDirOnPath -ExecutablePath $npmPath

    if (-not (Test-Path -LiteralPath (Join-Path $RootDir "conf.py"))) {
        throw ("conf.py not found at {0}" -f (Join-Path $RootDir "conf.py"))
    }
    if (-not (Test-Path -LiteralPath $FrontendDir)) {
        throw ("sau_frontend directory not found at {0}" -f $FrontendDir)
    }

    Remove-Item -LiteralPath $LauncherPidFile -Force -ErrorAction SilentlyContinue
    Start-Backend -PythonSpec $pythonSpec
    Start-Frontend -NpmPath $npmPath
    Write-Log -Level "INFO" -Message ("Detached SAU stack started backend_pid={0} frontend_pid={1}" -f (Get-StoredPid -PidFile $BackendPidFile), (Get-StoredPid -PidFile $FrontendPidFile))
}

function Stop-Daemon {
    Stop-ProcessFromFile -Name "SAU stack launcher" -PidFile $LauncherPidFile
    Stop-ProcessFromFile -Name "SAU frontend" -PidFile $FrontendPidFile
    Stop-ProcessFromFile -Name "SAU backend" -PidFile $BackendPidFile
    Stop-ProcessByPort -Name "SAU frontend" -Port $FrontendPort -PidFile $FrontendPidFile
    Stop-ProcessByPort -Name "SAU backend" -Port $BackendPort -PidFile $BackendPidFile
}

function Show-Status {
    Remove-StalePidFile -PidFile $LauncherPidFile
    Remove-StalePidFile -PidFile $BackendPidFile
    Remove-StalePidFile -PidFile $FrontendPidFile

    $status = [ordered]@{
        launcherPid = Get-StoredPid -PidFile $LauncherPidFile
        backendPid = Get-StoredPid -PidFile $BackendPidFile
        frontendPid = Get-StoredPid -PidFile $FrontendPidFile
        launcherLog = $LauncherLogFile
        launcherErrorLog = $LauncherErrorLogFile
        backendLog = $BackendLogFile
        backendErrorLog = $BackendErrorLogFile
        frontendLog = $FrontendLogFile
        frontendErrorLog = $FrontendErrorLogFile
        envFile = $EnvFile
    }
    $status | ConvertTo-Json -Depth 3
}

function Install-StartupTask {
    $userId = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
    $startupTaskName = "{0} Startup" -f $TaskName
    $cmdPath = (Get-Command "cmd.exe" -ErrorAction Stop).Source
    $startScript = Join-Path $RootDir "start-win.bat"
    $arguments = '/c ""{0}""' -f $startScript

    $taskAction = New-ScheduledTaskAction -Execute $cmdPath -Argument $arguments
    $taskSettings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -MultipleInstances IgnoreNew

    $logonTrigger = New-ScheduledTaskTrigger -AtLogOn
    $logonPrincipal = New-ScheduledTaskPrincipal -UserId $userId -LogonType Interactive -RunLevel Highest
    $logonTask = New-ScheduledTask -Action $taskAction -Trigger $logonTrigger -Principal $logonPrincipal -Settings $taskSettings
    Register-ScheduledTask -TaskName $TaskName -InputObject $logonTask -Force | Out-Null
    Write-Log -Level "INFO" -Message ("Installed Windows logon task '{0}' for {1}" -f $TaskName, $userId)

    $startupTrigger = New-ScheduledTaskTrigger -AtStartup
    $startupPrincipal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    $startupTask = New-ScheduledTask -Action $taskAction -Trigger $startupTrigger -Principal $startupPrincipal -Settings $taskSettings
    Register-ScheduledTask -TaskName $startupTaskName -InputObject $startupTask -Force | Out-Null
    Write-Log -Level "INFO" -Message ("Installed Windows startup task '{0}' as SYSTEM" -f $startupTaskName)
}

function Uninstall-StartupTask {
    $taskNames = @(
        $TaskName,
        ("{0} Startup" -f $TaskName)
    )

    $removedAny = $false
    foreach ($name in $taskNames) {
        if (Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue) {
            Unregister-ScheduledTask -TaskName $name -Confirm:$false
            Write-Log -Level "INFO" -Message ("Removed Windows scheduled task '{0}'" -f $name)
            $removedAny = $true
        }
    }

    if (-not $removedAny) {
        Write-Log -Level "INFO" -Message ("Windows scheduled tasks for '{0}' were not present" -f $TaskName)
    }
}

switch ($Action) {
    "run" { Start-RunLoop }
    "start" { Start-Daemon }
    "stop" { Stop-Daemon }
    "restart" {
        Stop-Daemon
        Start-Daemon
    }
    "status" { Show-Status }
    "install-task" { Install-StartupTask }
    "uninstall-task" { Uninstall-StartupTask }
}
