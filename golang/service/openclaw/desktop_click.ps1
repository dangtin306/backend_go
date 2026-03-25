param(
  [int]$X,
  [int]$Y,
  [string]$Button = "left",
  [string]$DoubleClick = "0"
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

$DoubleClickBool = Convert-ToOpenClawBool $DoubleClick

Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class OpenClawUser32Click {
  [DllImport("user32.dll")]
  public static extern bool SetCursorPos(int X, int Y);
  [DllImport("user32.dll")]
  public static extern void mouse_event(uint dwFlags, uint dx, uint dy, uint dwData, UIntPtr dwExtraInfo);
}
"@

function Invoke-MouseClick {
  param([string]$Kind)
  switch ($Kind.ToLowerInvariant()) {
    "left" {
      [OpenClawUser32Click]::mouse_event(0x0002, 0, 0, 0, [UIntPtr]::Zero)
      Start-Sleep -Milliseconds 35
      [OpenClawUser32Click]::mouse_event(0x0004, 0, 0, 0, [UIntPtr]::Zero)
    }
    "right" {
      [OpenClawUser32Click]::mouse_event(0x0008, 0, 0, 0, [UIntPtr]::Zero)
      Start-Sleep -Milliseconds 35
      [OpenClawUser32Click]::mouse_event(0x0010, 0, 0, 0, [UIntPtr]::Zero)
    }
    "middle" {
      [OpenClawUser32Click]::mouse_event(0x0020, 0, 0, 0, [UIntPtr]::Zero)
      Start-Sleep -Milliseconds 35
      [OpenClawUser32Click]::mouse_event(0x0040, 0, 0, 0, [UIntPtr]::Zero)
    }
    default {
      throw "unsupported button: $Kind"
    }
  }
}

[OpenClawUser32Click]::SetCursorPos($X, $Y) | Out-Null
Start-Sleep -Milliseconds 80
Invoke-MouseClick -Kind $Button
if ($DoubleClickBool) {
  Start-Sleep -Milliseconds 120
  Invoke-MouseClick -Kind $Button
}

[ordered]@{
  ok = $true
  x = $X
  y = $Y
  button = $Button
  doubleClick = $DoubleClickBool
} | ConvertTo-Json -Depth 5 -Compress
