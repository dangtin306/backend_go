param()

$ahkCandidates = @(
  "C:\Program Files\AutoHotkey\AutoHotkey.exe",
  "C:\Program Files\AutoHotkey\v2\AutoHotkey64.exe",
  "C:\Program Files\AutoHotkey\v2\AutoHotkey.exe",
  "C:\Program Files\AutoHotkey\UX\AutoHotkeyUX.exe"
)

$ahkPaths = @()
foreach ($candidate in $ahkCandidates) {
  if (Test-Path $candidate) {
    $ahkPaths += $candidate
  }
}

$commandAhk = Get-Command AutoHotkey.exe -ErrorAction SilentlyContinue
if ($commandAhk -and $commandAhk.Source) {
  $ahkPaths += $commandAhk.Source
}

$windows = @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
  $_.MainWindowHandle -ne 0 -and -not [string]::IsNullOrWhiteSpace($_.MainWindowTitle)
})

$explorer = @(Get-Process explorer -ErrorAction SilentlyContinue)

$payload = [ordered]@{
  ok = $true
  userInteractive = [Environment]::UserInteractive
  sessionName = $env:SESSIONNAME
  currentUser = [Security.Principal.WindowsIdentity]::GetCurrent().Name
  explorerCount = $explorer.Count
  visibleWindows = $windows.Count
  autoHotkeyPaths = @($ahkPaths | Select-Object -Unique)
}

$payload | ConvertTo-Json -Depth 5 -Compress
