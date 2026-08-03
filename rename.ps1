<#
.SYNOPSIS
    Rewrites this template's module path and project name across the repository.

.DESCRIPTION
    The PowerShell equivalent of `make rename`, for Windows without Git Bash.
    Every other Makefile target is a plain `go ...` or `docker ...` invocation that
    works unchanged in any shell — this is the only one that depends on Unix tools
    (grep, xargs, sed), so it is the only one worth porting.

    Two passes, and the order matters: the full module path first, then the bare
    project name. Doing the bare name first would rewrite the tail of the module
    path and leave the full-path pass with nothing to match.

    The bare name is rewritten separately because it appears on its own where the
    module path does not reach — the Fiber AppName, for instance. Replacing only
    the module path would leave the new service announcing itself under the
    template's name.

.PARAMETER ModuleNew
    The new module path, e.g. github.com/acme/billing-api

.PARAMETER DryRun
    List the files that would change, and change nothing.

.EXAMPLE
    .\rename.ps1 -ModuleNew github.com/acme/billing-api

.EXAMPLE
    .\rename.ps1 -ModuleNew github.com/acme/billing-api -DryRun

.NOTES
    Keep behaviour aligned with the `rename` target in the Makefile. Both end with
    the same self-check — zero remaining occurrences — so a divergence in how they
    get there still fails loudly rather than silently leaving a half-renamed repo.

    Written for Windows PowerShell 5.1 as well as PowerShell 7+: no ternary
    operator, no null-coalescing, no `-Raw`/`-NoNewline` round-trips.
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ModuleNew,

    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = $PSScriptRoot

# Directories never touched. .git above all: rewriting object files would corrupt
# the repository, and -Force below would otherwise walk into it.
$excludedDirPattern = '[\\/](\.git|bin|dist|node_modules|vendor)[\\/]'

function Get-CurrentModulePath {
    <#
        Read the module path from go.mod rather than from `go list -m`.

        `go list -m` needs a working toolchain and a resolvable module graph; go.mod
        is a text file that is correct even when the build is temporarily broken —
        which is a plausible state for someone who just cloned a template.
    #>
    $goMod = Join-Path $repoRoot 'go.mod'
    if (-not (Test-Path -LiteralPath $goMod)) {
        throw "go.mod not found next to this script ($goMod). Run it from the repository root."
    }

    foreach ($line in [System.IO.File]::ReadAllLines($goMod)) {
        $trimmed = $line.Trim()
        if ($trimmed.StartsWith('module ')) {
            return $trimmed.Substring(7).Trim()
        }
    }

    throw "No 'module' directive found in $goMod."
}

function Test-IsBinaryFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    # A NUL byte in the first few KB is the same heuristic grep uses to decide a
    # file is binary. Rewriting one would corrupt it silently.
    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $buffer = New-Object byte[] 8000
        $read = $stream.Read($buffer, 0, $buffer.Length)
        for ($i = 0; $i -lt $read; $i++) {
            if ($buffer[$i] -eq 0) { return $true }
        }
    }
    finally {
        $stream.Dispose()
    }

    return $false
}

function Invoke-Replacement {
    <#
        Replace every literal occurrence of $Old with $New across the repository.
        Returns the list of paths that changed.
    #>
    param(
        [Parameter(Mandatory = $true)][string]$Old,
        [Parameter(Mandatory = $true)][string]$New,
        [Parameter(Mandatory = $true)][bool]$Apply
    )

    $changed = @()

    # -Force so dotfiles are included: .golangci.yml, .env.example and everything
    # under .claude/ carry the module path too.
    $files = Get-ChildItem -LiteralPath $repoRoot -Recurse -File -Force |
        Where-Object { $_.FullName -notmatch $excludedDirPattern }

    foreach ($file in $files) {
        if (Test-IsBinaryFile -Path $file.FullName) { continue }

        # ReadAllText / WriteAllText round-trip the bytes as they are, so LF line
        # endings stay LF. That is not cosmetic: the hooks under .claude/hooks are
        # bash scripts, and CRLF in them makes Git Bash fail with "\r: command not
        # found" — the guardrails would break at the exact moment nobody is looking.
        $content = [System.IO.File]::ReadAllText($file.FullName)

        # .Replace(), not -replace: the latter takes a regular expression, where the
        # dots in "github.com" match any character. Literal replacement is both
        # correct and what is meant here.
        $updated = $content.Replace($Old, $New)

        if ($updated -ne $content) {
            $changed += $file.FullName

            if ($Apply) {
                # UTF8Encoding($false) => no BOM. Set-Content and Out-File would add
                # one on Windows PowerShell 5.1, and a BOM in a .go or .sh file
                # breaks the toolchain that reads it.
                $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
                [System.IO.File]::WriteAllText($file.FullName, $updated, $utf8NoBom)
            }
        }
    }

    return $changed
}

function Assert-NoRemainingOccurrence {
    <#
        Post-condition check. This is the part that makes the two implementations —
        this script and the Makefile target — interchangeable: whatever route they
        take, a repository that still mentions the old name is a failed rename, and
        it fails loudly instead of leaving a half-renamed tree to discover later.
    #>
    param([Parameter(Mandatory = $true)][string[]]$Needles)

    $offenders = @()

    $files = Get-ChildItem -LiteralPath $repoRoot -Recurse -File -Force |
        Where-Object { $_.FullName -notmatch $excludedDirPattern }

    foreach ($file in $files) {
        if (Test-IsBinaryFile -Path $file.FullName) { continue }

        $content = [System.IO.File]::ReadAllText($file.FullName)
        foreach ($needle in $Needles) {
            if ($content.Contains($needle)) {
                $offenders += "$($file.FullName) (still contains '$needle')"
                break
            }
        }
    }

    if ($offenders.Count -gt 0) {
        Write-Host ''
        Write-Host 'Rename incomplete — these files still mention the old name:' -ForegroundColor Red
        $offenders | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        throw 'Rename did not reach every occurrence.'
    }
}

# --- main ---------------------------------------------------------------------

$moduleOld = Get-CurrentModulePath
$nameOld = $moduleOld.Split('/')[-1]
$nameNew = $ModuleNew.Split('/')[-1]

if ($ModuleNew -notmatch '^[A-Za-z0-9._~-]+(\.[A-Za-z0-9._~-]+)+/[A-Za-z0-9._~/-]+$') {
    throw "ModuleNew '$ModuleNew' does not look like a module path (expected e.g. github.com/acme/billing-api)."
}

if ($ModuleNew -eq $moduleOld) {
    Write-Host "Module path is already '$ModuleNew'. Nothing to do." -ForegroundColor Yellow
    return
}

Write-Host "Renaming $moduleOld -> $ModuleNew"
Write-Host "      and $nameOld -> $nameNew"

if ($DryRun) {
    Write-Host '(dry run — no file will be modified)' -ForegroundColor Yellow
}

$apply = -not $DryRun

# Full module path first. See the note at the top of this file for why the order
# is not interchangeable.
$changedModule = Invoke-Replacement -Old $moduleOld -New $ModuleNew -Apply $apply
$changedName = @()

if ($apply) {
    $changedName = Invoke-Replacement -Old $nameOld -New $nameNew -Apply $true
}
else {
    # In dry-run the first pass changed nothing, so the bare name is still part of
    # the module path everywhere: a second scan would report every file again and
    # tell the user nothing. Report the first pass only, and say so.
    Write-Host 'Dry run reports the module-path pass only; the bare-name pass runs on the real thing.' -ForegroundColor Yellow
}

$touched = @($changedModule + $changedName | Sort-Object -Unique)

Write-Host ''
Write-Host "$($touched.Count) file(s) $(if ($DryRun) { 'would change' } else { 'changed' }):"
$touched | ForEach-Object { Write-Host "  $($_.Substring($repoRoot.Length + 1))" }

if ($DryRun) {
    return
}

Write-Host ''
Write-Host 'Running go mod tidy...'
& go mod tidy
if ($LASTEXITCODE -ne 0) {
    # PowerShell does not stop on a failing native command, so the exit code has to
    # be checked explicitly. Without this the script would report success on a
    # broken module — a silent green, which is worse than a loud failure.
    throw "go mod tidy failed with exit code $LASTEXITCODE."
}

Assert-NoRemainingOccurrence -Needles @($moduleOld, $nameOld)

Write-Host ''
Write-Host 'Done. Run the checks to confirm:' -ForegroundColor Green
Write-Host '  go vet ./...'
Write-Host '  go test ./...'
Write-Host '  golangci-lint run ./...'
Write-Host ''
Write-Host 'Note: README.md and CLAUDE.md still describe the template — /bootstrap-spec rewrites their headers.'
