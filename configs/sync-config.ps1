#!/usr/bin/env pwsh

<#
.SYNOPSIS
Sync YAML structure and comments from config.example.yaml to config.yaml while preserving existing values.

.DESCRIPTION
This script reads the structure and comments from config.example.yaml and applies them to config.yaml,
but preserves any existing values that are already set in config.yaml.

.PARAMETER ExampleFile
Path to the example config file (default: config.example.yaml)

.PARAMETER ConfigFile
Path to the target config file (default: config.yaml)

.PARAMETER BackupSuffix
Suffix for backup file (default: .bak)

.EXAMPLE
.\sync-config.ps1
.\sync-config.ps1 -ExampleFile "config.example.yaml" -ConfigFile "config.yaml"
#>

param(
    [string]$ExampleFile = "config.example.yaml",
    [string]$ConfigFile = "config.yaml",
    [string]$BackupSuffix = ".bak"
)

# Function to check if a line contains a meaningful value (not empty, not just comments)
function Test-HasValue {
    param([string]$line)
    
    $trimmed = $line.Trim()
    
    # Skip empty lines, comments, or lines with just key without value
    if ([string]::IsNullOrWhiteSpace($trimmed) -or 
        $trimmed.StartsWith('#') -or 
        $trimmed.EndsWith(':') -or
        $trimmed.Contains(': #') -or
        $trimmed.Contains(':  #') -or
        ($trimmed.Contains(':') -and $trimmed.Split(':')[1].Trim().StartsWith('#'))) {
        return $false
    }
    
    # Check if line has a value after the colon
    if ($trimmed.Contains(':')) {
        $parts = $trimmed.Split(':', 2)
        if ($parts.Length -eq 2) {
            $value = $parts[1].Trim()
            # Remove inline comments
            if ($value.Contains('#')) {
                $value = $value.Split('#')[0].Trim()
            }
            return -not [string]::IsNullOrWhiteSpace($value) -and $value -ne '""' -and $value -ne "''"
        }
    }
    
    return $false
}

# Function to extract key from YAML line
function Get-YamlKey {
    param([string]$line)
    
    $trimmed = $line.Trim()
    if ($trimmed.Contains(':')) {
        return $trimmed.Split(':')[0].Trim()
    }
    return $null
}

# Function to get indentation level
function Get-IndentLevel {
    param([string]$line)
    
    $indent = 0
    foreach ($char in $line.ToCharArray()) {
        if ($char -eq ' ') {
            $indent++
        } elseif ($char -eq "`t") {
            $indent += 2  # Treat tab as 2 spaces
        } else {
            break
        }
    }
    return $indent
}

# Function to build key path for nested YAML
function Get-KeyPath {
    param([string[]]$lines, [int]$currentIndex)
    
    $path = @()
    $currentIndent = Get-IndentLevel $lines[$currentIndex]
    $currentKey = Get-YamlKey $lines[$currentIndex]
    
    if ($currentKey) {
        $path += $currentKey
    }
    
    # Go backwards to find parent keys
    for ($i = $currentIndex - 1; $i -ge 0; $i--) {
        $lineIndent = Get-IndentLevel $lines[$i]
        $key = Get-YamlKey $lines[$i]
        
        if ($key -and $lineIndent -lt $currentIndent) {
            $path = @($key) + $path
            $currentIndent = $lineIndent
        }
    }
    
    return $path -join '.'
}

# Main script
try {
    Write-Host "Starting YAML config synchronization..." -ForegroundColor Green
    
    # Check if files exist
    if (-not (Test-Path $ExampleFile)) {
        throw "Example file '$ExampleFile' not found!"
    }
    
    if (-not (Test-Path $ConfigFile)) {
        Write-Warning "Config file '$ConfigFile' not found. Creating from example..."
        Copy-Item $ExampleFile $ConfigFile
        Write-Host "Created '$ConfigFile' from '$ExampleFile'" -ForegroundColor Green
        exit 0
    }
    
    # Create backup
    $backupFile = $ConfigFile -replace '\.yaml$', '.bak.yaml'
    Copy-Item $ConfigFile $backupFile -Force
    Write-Host "Created backup: $backupFile" -ForegroundColor Yellow
    
    # Read files
    $exampleLines = Get-Content $ExampleFile -Encoding UTF8
    $configLines = Get-Content $ConfigFile -Encoding UTF8
    
    # Build map of existing values in config file
    $existingValues = @{}
    for ($i = 0; $i -lt $configLines.Length; $i++) {
        $line = $configLines[$i]
        if (Test-HasValue $line) {
            $keyPath = Get-KeyPath $configLines $i
            if ($keyPath) {
                $existingValues[$keyPath] = $line
                Write-Verbose "Found existing value: $keyPath = $($line.Trim())"
            }
        }
    }
    
    Write-Host "Found $($existingValues.Count) existing values to preserve" -ForegroundColor Cyan
    
    # Process example file and merge with existing values
    $outputLines = @()
    $inCommentBlock = $false
    
    for ($i = 0; $i -lt $exampleLines.Length; $i++) {
        $line = $exampleLines[$i]
        $trimmed = $line.Trim()
        
        # Always preserve comment blocks and empty lines
        if ([string]::IsNullOrWhiteSpace($trimmed) -or $trimmed.StartsWith('#')) {
            $outputLines += $line
            continue
        }
        
        # Check if this line has a value in the example
        if (Test-HasValue $line) {
            $keyPath = Get-KeyPath $exampleLines $i
            
            # If we have an existing value for this key, use it instead
            if ($keyPath -and $existingValues.ContainsKey($keyPath)) {
                $existingLine = $existingValues[$keyPath]
                $indent = $line.Substring(0, (Get-IndentLevel $line))
                
                # Preserve the indentation from example but use existing value
                $key = Get-YamlKey $line
                $existingValue = $existingLine.Split(':', 2)[1].Trim()
                
                # Handle inline comments from example
                $inlineComment = ""
                if ($line.Contains('#')) {
                    $commentPart = $line.Split('#', 2)[1]
                    $inlineComment = " # $commentPart"
                }
                
                $newLine = "$indent$key`: $existingValue$inlineComment"
                $outputLines += $newLine
                Write-Host "Preserved value for '$keyPath': $existingValue" -ForegroundColor Green
            } else {
                # No existing value, use example line as-is
                $outputLines += $line
            }
        } else {
            # Line without value (structure only), use as-is
            $outputLines += $line
        }
    }
    
    # Write the merged content
    $outputLines | Out-File -FilePath $ConfigFile -Encoding UTF8 -Force
    
    Write-Host "`nSynchronization completed successfully!" -ForegroundColor Green
    Write-Host "- Structure and comments updated from: $ExampleFile" -ForegroundColor White
    Write-Host "- Existing values preserved in: $ConfigFile" -ForegroundColor White
    Write-Host "- Backup created: $backupFile" -ForegroundColor White
    
    # Show summary
    $newLines = Get-Content $ConfigFile -Encoding UTF8
    $totalLines = $newLines.Length
    $preservedCount = $existingValues.Count
    
    Write-Host "`nSummary:" -ForegroundColor Cyan
    Write-Host "- Total lines: $totalLines" -ForegroundColor White
    Write-Host "- Preserved values: $preservedCount" -ForegroundColor White
    Write-Host "- Backup file: $backupFile" -ForegroundColor White
    
} catch {
    Write-Error "Error during synchronization: $($_.Exception.Message)"
    exit 1
} 