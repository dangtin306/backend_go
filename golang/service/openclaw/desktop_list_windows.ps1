param(
  [string]$Title = "",
  [int]$Limit = 50
)

Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class OpenClawUser32List {
  [StructLayout(LayoutKind.Sequential)]
  public struct RECT {
    public int Left;
    public int Top;
    public int Right;
    public int Bottom;
  }
  [DllImport("user32.dll")]
  public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
  [DllImport("user32.dll")]
  public static extern bool IsWindowVisible(IntPtr hWnd);
}
"@

$items = Get-Process -ErrorAction SilentlyContinue | Where-Object {
  $_.MainWindowHandle -ne 0 -and
  -not [string]::IsNullOrWhiteSpace($_.MainWindowTitle)
}

if (-not [string]::IsNullOrWhiteSpace($Title)) {
  $items = $items | Where-Object { $_.MainWindowTitle -like "*$Title*" }
}

$rows = @()
foreach ($proc in $items | Sort-Object StartTime -Descending | Select-Object -First $Limit) {
  $rect = New-Object OpenClawUser32List+RECT
  $null = [OpenClawUser32List]::GetWindowRect([IntPtr]$proc.MainWindowHandle, [ref]$rect)
  $rows += [ordered]@{
    processId = $proc.Id
    processName = $proc.ProcessName
    title = $proc.MainWindowTitle
    hwnd = [int64]$proc.MainWindowHandle
    visible = [OpenClawUser32List]::IsWindowVisible([IntPtr]$proc.MainWindowHandle)
    left = $rect.Left
    top = $rect.Top
    width = ($rect.Right - $rect.Left)
    height = ($rect.Bottom - $rect.Top)
  }
}

[ordered]@{
  ok = $true
  count = $rows.Count
  windows = $rows
} | ConvertTo-Json -Depth 6 -Compress
