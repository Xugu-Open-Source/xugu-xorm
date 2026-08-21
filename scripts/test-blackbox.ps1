param(
    [Parameter(Mandatory = $true)]
    [string]$ModuleVersion,
    [string]$ModulePath = "github.com/Xugu-Open-Source/xugu-xorm",
    [string]$DriverVersion = "v1.0.13",
    [string]$CandidateRepository
)

$ErrorActionPreference = "Stop"

function Invoke-Go {
    param(
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$Arguments
    )

    & go @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "go $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is required to run the black-box consumer test."
}

$template = Join-Path $PSScriptRoot "..\test\blackbox\consumer_test.go.tmpl"
$workdir = Join-Path ([System.IO.Path]::GetTempPath()) ("xugu-xorm-blackbox-" + [guid]::NewGuid())
$previousGoWork = $env:GOWORK
$previousGoRoot = $env:GOROOT
$previousGoProxy = $env:GOPROXY
$previousGoSumDB = $env:GOSUMDB
$previousGitConfigCount = $env:GIT_CONFIG_COUNT
$previousGitConfigKey0 = $env:GIT_CONFIG_KEY_0
$previousGitConfigValue0 = $env:GIT_CONFIG_VALUE_0
$hadGoWork = Test-Path Env:GOWORK
$hadGoRoot = Test-Path Env:GOROOT
$hadGoProxy = Test-Path Env:GOPROXY
$hadGoSumDB = Test-Path Env:GOSUMDB
$hadGitConfigCount = Test-Path Env:GIT_CONFIG_COUNT
$hadGitConfigKey0 = Test-Path Env:GIT_CONFIG_KEY_0
$hadGitConfigValue0 = Test-Path Env:GIT_CONFIG_VALUE_0
$pushedLocation = $false

try {
    New-Item -ItemType Directory -Path $workdir | Out-Null
    Copy-Item -LiteralPath $template -Destination (Join-Path $workdir "consumer_test.go")

    Push-Location $workdir
	$pushedLocation = $true
    $env:GOWORK = "off"
    Remove-Item Env:GOROOT -ErrorAction SilentlyContinue
    if (-not $hadGoProxy) {
        $env:GOPROXY = "https://goproxy.cn"
    }
    if (-not $hadGoSumDB) {
        $env:GOSUMDB = "off"
    }
    if ($CandidateRepository) {
        $candidatePath = (Resolve-Path -LiteralPath $CandidateRepository).Path.Replace("\", "/")
        $env:GIT_CONFIG_COUNT = "1"
        $env:GIT_CONFIG_KEY_0 = "url.file:///$candidatePath.insteadOf"
        $env:GIT_CONFIG_VALUE_0 = "https://github.com/Xugu-Open-Source/xugu-xorm"
        $env:GOPROXY = "direct"
        $env:GOSUMDB = "off"
    }
    if ($env:XUGU_IT -eq "1" -and [string]::IsNullOrWhiteSpace($env:XUGU_TEST_DSN)) {
        throw "XUGU_TEST_DSN is required when XUGU_IT=1"
    }
    Invoke-Go mod init "example.com/xugu-xorm-blackbox"
    Invoke-Go get "$ModulePath@$ModuleVersion"
    Invoke-Go get "gitee.com/XuguDB/go-xugu-driver@$DriverVersion"
    Invoke-Go mod tidy
    Invoke-Go test -count=1 .
} finally {
	if ($pushedLocation) {
		Pop-Location
	}
	if ($hadGoWork) {
		$env:GOWORK = $previousGoWork
	} else {
		Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
	}
	if ($hadGoRoot) {
		$env:GOROOT = $previousGoRoot
	} else {
		Remove-Item Env:GOROOT -ErrorAction SilentlyContinue
	}
	if ($hadGoProxy) {
		$env:GOPROXY = $previousGoProxy
	} else {
		Remove-Item Env:GOPROXY -ErrorAction SilentlyContinue
	}
    if ($hadGoSumDB) {
        $env:GOSUMDB = $previousGoSumDB
    } else {
        Remove-Item Env:GOSUMDB -ErrorAction SilentlyContinue
    }
    if ($hadGitConfigCount) {
        $env:GIT_CONFIG_COUNT = $previousGitConfigCount
    } else {
        Remove-Item Env:GIT_CONFIG_COUNT -ErrorAction SilentlyContinue
    }
    if ($hadGitConfigKey0) {
        $env:GIT_CONFIG_KEY_0 = $previousGitConfigKey0
    } else {
        Remove-Item Env:GIT_CONFIG_KEY_0 -ErrorAction SilentlyContinue
    }
    if ($hadGitConfigValue0) {
        $env:GIT_CONFIG_VALUE_0 = $previousGitConfigValue0
    } else {
        Remove-Item Env:GIT_CONFIG_VALUE_0 -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $workdir -Recurse -Force -ErrorAction SilentlyContinue
}
