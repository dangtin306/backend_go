param(
  [string]$Title = "",
  [string]$ProcessName = "",
  [string]$Exact = "0"
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

Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class OpenClawUser32Activate {
  [DllImport("user32.dll")]
  public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")]
  public static extern bool ShowWindowAsync(IntPtr hWnd, int nCmdShow);
  [DllImport("user32.dll")]
  public static extern bool IsIconic(IntPtr hWnd);
}
"@

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

$hwnd = [IntPtr]$proc.MainWindowHandle
if ([OpenClawUser32Activate]::IsIconic($hwnd)) {
  [OpenClawUser32Activate]::ShowWindowAsync($hwnd, 9) | Out-Null
  Start-Sleep -Milliseconds 200
}

$activated = [OpenClawUser32Activate]::SetForegroundWindow($hwnd)
if (-not $activated) {
  $wshell = New-Object -ComObject WScript.Shell
  $activated = $wshell.AppActivate($proc.Id)
}

[ordered]@{
  ok = [bool]$activated
  processId = $proc.Id
  processName = $proc.ProcessName
  title = $proc.MainWindowTitle
  hwnd = [int64]$proc.MainWindowHandle
} | ConvertTo-Json -Depth 5 -Compress
