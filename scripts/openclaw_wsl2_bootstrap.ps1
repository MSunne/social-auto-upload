param(
    [string]$DistroName = "Ubuntu-24.04",
    [string]$BootTaskName = "OpenClaw WSL Boot",
    [switch]$InstallBootTask = $true
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = [System.IO.Path]::GetFullPath((Join-Path $ScriptDir ".."))

function Write-Log {
    param([string]$Message)
    "[{0}] [openclaw-wsl2] {1}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Message
}

function Assert-WslInstalled {
    if ($null -eq (Get-Command "wsl.exe" -ErrorAction SilentlyContinue)) {
        throw "wsl.exe not found. Install WSL2 first."
    }
}

function Get-WslDistroList {
    $output = & wsl.exe --list --quiet
    return @($output | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
}

function Set-IniValue {
    param(
        [string]$Path,
        [string]$Section,
        [string]$Key,
        [string]$Value
    )

    $lines = @()
    if (Test-Path -LiteralPath $Path) {
        $lines = Get-Content -LiteralPath $Path
    }

    $result = New-Object System.Collections.Generic.List[string]
    $sectionHeader = "[{0}]" -f $Section
    $sectionFound = $false
    $keyWritten = $false
    $inSection = $false

    foreach ($line in $lines) {
        $trimmed = $line.Trim()
        if ($trimmed -match '^\[.+\]$') {
            if ($inSection -and -not $keyWritten) {
                $result.Add("{0}={1}" -f $Key, $Value)
                $keyWritten = $true
            }
            $inSection = ($trimmed -ieq $sectionHeader)
            if ($inSection) {
                $sectionFound = $true
            }
            $result.Add($line)
            continue
        }

        if ($inSection -and ($trimmed -match ('^{0}\s*=' -f [regex]::Escape($Key)))) {
            if (-not $keyWritten) {
                $result.Add("{0}={1}" -f $Key, $Value)
                $keyWritten = $true
            }
            continue
        }

        $result.Add($line)
    }

    if (-not $sectionFound) {
        if ($result.Count -gt 0 -and -not [string]::IsNullOrWhiteSpace($result[$result.Count - 1])) {
            $result.Add("")
        }
        $result.Add($sectionHeader)
        $result.Add("{0}={1}" -f $Key, $Value)
    }
    elseif (-not $keyWritten) {
        $result.Add("{0}={1}" -f $Key, $Value)
    }

    Set-Content -LiteralPath $Path -Value $result -Encoding ASCII
}

function Invoke-WslScript {
    param(
        [string]$Script,
        [switch]$AsRoot
    )

    $command = @("wsl.exe", "-d", $DistroName)
    if ($AsRoot) {
        $command += @("-u", "root")
    }
    $command += @("--", "bash", "-s")

    $tempFile = [System.IO.Path]::GetTempFileName()
    try {
        Set-Content -LiteralPath $tempFile -Value $Script -Encoding UTF8
        $commandExe = $command[0]
        $commandArgs = @($command[1..($command.Length - 1)])
        Get-Content -LiteralPath $tempFile -Raw | & $commandExe @commandArgs
        if ($LASTEXITCODE -ne 0) {
            throw ("WSL script failed with exit code {0}" -f $LASTEXITCODE)
        }
    }
    finally {
        Remove-Item -LiteralPath $tempFile -Force -ErrorAction SilentlyContinue
    }
}

Assert-WslInstalled

$distros = Get-WslDistroList
if ($distros -notcontains $DistroName) {
    throw ("WSL distro '{0}' not found. Installed distros: {1}" -f $DistroName, ($distros -join ", "))
}

$wslConfigPath = Join-Path $HOME ".wslconfig"
Set-IniValue -Path $wslConfigPath -Section "wsl2" -Key "networkingMode" -Value "mirrored"
Set-IniValue -Path $wslConfigPath -Section "wsl2" -Key "localhostForwarding" -Value "true"
Write-Log ("Updated {0}" -f $wslConfigPath)

$repoWslPath = (& wsl.exe -d $DistroName -- wslpath -a $RootDir).Trim()
if ([string]::IsNullOrWhiteSpace($repoWslPath)) {
    throw "Failed to resolve the repository path inside WSL."
}

$rootScript = @'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y curl xz-utils ca-certificates git python3
cat >/etc/wsl.conf <<'EOF'
[boot]
systemd=true
EOF
'@

Invoke-WslScript -Script $rootScript -AsRoot

Write-Log "Restarting WSL to apply mirrored networking and systemd"
& wsl.exe --shutdown
Start-Sleep -Seconds 3

$defaultUser = (& wsl.exe -d $DistroName -- whoami).Trim()
if ([string]::IsNullOrWhiteSpace($defaultUser)) {
    throw "Could not resolve the default WSL user."
}

$userScript = @"
set -euo pipefail
mkdir -p "\$HOME/.local" "\$HOME/.npm-global"
if ! command -v node >/dev/null 2>&1 || ! node -e 'process.exit(Number(process.versions.node.split(".")[0]) >= 22 ? 0 : 1)'; then
  temp_dir="\$(mktemp -d)"
  trap 'rm -rf "\$temp_dir"' EXIT
  cd "\$temp_dir"
  arch="\$(uname -m)"
  case "\$arch" in
    x86_64|amd64) node_arch="x64" ;;
    aarch64|arm64) node_arch="arm64" ;;
    *) echo "Unsupported WSL architecture: \$arch" >&2; exit 1 ;;
  esac
  node_index="\$(curl -fsSL https://nodejs.org/dist/latest-v22.x/SHASUMS256.txt)"
  node_prefix="\$(printf '%s\n' "\$node_index" | awk -v arch="\$node_arch" '\$2 ~ ("linux-" arch "\\\\.tar\\\\.xz$") { sub("-linux-" arch "\\\\.tar\\\\.xz$", "", \$2); print \$2; exit }')"
  curl -fsSL "https://nodejs.org/dist/latest-v22.x/\${node_prefix}-linux-\${node_arch}.tar.xz" | tar -xJf - -C "\$HOME/.local"
  ln -sfn "\$HOME/.local/\${node_prefix}-linux-\${node_arch}" "\$HOME/.local/node-current"
fi
if ! grep -q '/.local/node-current/bin' "\$HOME/.profile" 2>/dev/null; then
  printf '\nexport PATH="\$HOME/.local/node-current/bin:\$PATH"\n' >> "\$HOME/.profile"
fi
if ! grep -q '/.npm-global/bin' "\$HOME/.profile" 2>/dev/null; then
  printf 'export PATH="\$HOME/.npm-global/bin:\$PATH"\n' >> "\$HOME/.profile"
fi
export PATH="\$HOME/.local/node-current/bin:\$HOME/.npm-global/bin:\$PATH"
npm config set prefix "\$HOME/.npm-global" >/dev/null 2>&1 || true
if [[ ! -x "\$HOME/.npm-global/bin/openclaw" ]]; then
  npm install -g openclaw@latest
fi
openclaw plugins install -l "{0}/openclaw_extensions/omnibull"
openclaw plugins enable omnibull
openclaw plugins install -l "{0}/openclaw_extensions/omnidrive"
openclaw plugins enable omnidrive
python3 - <<'PY'
import json
from pathlib import Path

path = Path.home() / ".openclaw" / "openclaw.json"
path.parent.mkdir(parents=True, exist_ok=True)
if path.exists():
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        data = {{}}
else:
    data = {{}}
gateway = data.get("gateway")
if not isinstance(gateway, dict):
    gateway = {{}}
gateway["mode"] = "local"
data["gateway"] = gateway
path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\\n", encoding="utf-8")
PY
openclaw gateway install
"@ -f $repoWslPath

Invoke-WslScript -Script $userScript

$lingerScript = @"
set -euo pipefail
if command -v loginctl >/dev/null 2>&1; then
  loginctl enable-linger "{0}" || true
fi
"@ -f $defaultUser

Invoke-WslScript -Script $lingerScript -AsRoot

if ($InstallBootTask) {
    Write-Log ("Installing Windows boot task '{0}'" -f $BootTaskName)
    & schtasks.exe /Create /F /SC ONSTART /RU SYSTEM /TN $BootTaskName /TR ('wsl.exe -d "{0}" --exec /bin/true' -f $DistroName) | Out-Null
}

Write-Log "WSL2 OpenClaw bootstrap completed."
