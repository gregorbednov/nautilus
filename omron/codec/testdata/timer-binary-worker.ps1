param([Parameter(Mandatory=$true)][string]$Directory)
$ErrorActionPreference='Stop'
if([IntPtr]::Size -ne 4){throw 'The library oracle requires x86 PowerShell'}
$cxDirectory='C:\Program Files (x86)\OMRON\CX-One\CX-Programmer'
$env:PATH=$cxDirectory+';'+$env:PATH
[IO.Directory]::SetCurrentDirectory($cxDirectory)
[IO.File]::WriteAllText((Join-Path $Directory 'worker-ready.txt'),[string]$PID)
$deadline=[DateTime]::UtcNow.AddSeconds(60)
while(-not (Test-Path -LiteralPath (Join-Path $Directory 'worker-stop.txt')) -and [DateTime]::UtcNow -lt $deadline){Start-Sleep -Milliseconds 100}
