param(
  [string]$Keys,
  [string]$Title = "",
  [string]$ProcessName = "",
  [string]$Exact = "0",
  [int]$DelayMs = 250
)

function Convert-ToOpenClawBool {
  param([object]$Value)
  $raw = [string]$Value
  if ([string]::IsNullOrWhiteSpace($raw)) { return $false }
  switch ($raw.Trim().ToLowerInvariant()) {
    "1" { return $true }
    "true" { return $true }
    "yes" { return $true }
    "on" { return $true }
    default { return $false }
  }
}

$ExactBool = Convert-ToOpenClawBool $Exact

$wshell = New-Object -ComObject WScript.Shell

if (-not [string]::IsNullOrWhiteSpace($Title) -or -not [string]::IsNullOrWhiteSpace($ProcessName)) {
  $items = Get-Process -ErrorAction SilentlyContinue | Where-Object {
    $_.MainWindowHandle -ne 0 -and
    -not [string]::IsNullOrWhiteSpace($_.MainWindowTitle)
  }

  if (-not [string]::IsNullOrWhiteSpace($ProcessName)) {
    $items = $items | Where-Object { $_.ProcessName -ieq $ProcessName }
  }
  if (-not [string]::IsNullOrWhiteSpace($Title)) {
    if ($ExactBool) {
      $items = $items | Where-Object { $_.MainWindowTitle -eq $Title }
    } else {
      $items = $items | Where-Object { $_.MainWindowTitle -like "*$Title*" }
    }
  }

  $proc = $items | Sort-Object StartTime -Descending | Select-Object -First 1
  if (-not $proc) {
    [ordered]@{
      ok = $false
      message = "window not found"
      title = $Title
      processName = $ProcessName
    } | ConvertTo-Json -Depth 5 -Compress
    exit 3
  }

  $activated = $wshell.AppActivate($proc.Id)
  if (-not $activated) {
    [ordered]@{
      ok = $false
      message = "failed to activate target window"
      processId = $proc.Id
      title = $proc.MainWindowTitle
    } | ConvertTo-Json -Depth 5 -Compress
    exit 4
  }
  Start-Sleep -Milliseconds $DelayMs
}

$wshell.SendKeys($Keys)

[ordered]@{
  ok = $true
  keys = $Keys
  title = $Title
  processName = $ProcessName
} | ConvertTo-Json -Depth 5 -Compress
