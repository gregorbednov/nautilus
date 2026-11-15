[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Cases,[Parameter(Mandatory=$true)][string]$Output)
$ErrorActionPreference='Stop'
$source=(Resolve-Path -LiteralPath $Cases).Path
if((Get-Item $source).Length -gt 1048576){throw 'Case input too large'}
$inputCases=@(Get-Content -LiteralPath $source -Raw|ConvertFrom-Json)
if($inputCases.Count -lt 1 -or $inputCases.Count -gt 128){throw 'Invalid case count'}
$ids=@{}
foreach($case in $inputCases){
 if($case.id -cnotmatch '^[a-z0-9-]+$' -or $ids.ContainsKey($case.id) -or $case.variables -isnot [array] -or $case.variables.Count -gt 256){throw 'Invalid case'}
 $ids[$case.id]=$true;$names=@{EN=$true;ENO=$true}
 foreach($variable in $case.variables){
  if($variable.name -cnotmatch '^[A-Za-z][A-Za-z0-9_]*$' -or $names.ContainsKey($variable.name) -or $variable.usage -notin @(0,1,2) -or $variable.typeCode -notin @(0,1,2,7,13) -or ($variable.typeCode -eq 13 -and $variable.usage -ne 0) -or $variable.initial -isnot [string]){throw 'Unqualified scalar declaration'}
  $names[$variable.name]=$true
 }
}
$destination=[IO.Path]::GetFullPath($Output)
if(Test-Path -LiteralPath $destination){throw 'Output directory must be new'}
$null=New-Item -ItemType Directory -Path $destination
Copy-Item -LiteralPath $source -Destination (Join-Path $destination 'cases.json')
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'allocation-worker.ps1') -Destination (Join-Path $destination 'worker.ps1')
$worker=Start-Process -FilePath 'C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe' -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-File',('"'+(Join-Path $destination 'worker.ps1')+'"'),'-Directory',('"'+$destination+'"')) -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $destination 'events.log') -RedirectStandardError (Join-Path $destination 'worker.stderr')
if(-not $worker.WaitForExit(30000)){Stop-Process -Id $worker.Id;throw 'Allocation oracle timeout'}
if($worker.ExitCode -ne 0 -or (Get-Item (Join-Path $destination 'worker.stderr')).Length -ne 0){throw 'Allocation worker failed; inspect output'}
$events=@(Get-Content (Join-Path $destination 'events.log')|ForEach-Object {if(-not $_.StartsWith('CJ1_ALLOCATION ')){throw 'Unexpected output'};$_.Substring(15)|ConvertFrom-Json})
if(@($events|Where-Object kind -eq 'error').Count -ne 0 -or @($events|Where-Object kind -eq 'case').Count -ne $inputCases.Count -or @($events|Where-Object kind -eq 'done').Count -ne 1){throw 'Incomplete allocation run'}
$provenance=[ordered]@{schemaVersion=1;method='com-pou-header-offline-allocation';pheadSHA256=$events[0].pheadSHA256;requestSHA256=$events[0].requestSHA256;eventIID=$events[0].eventIID;onlineBoolReserve=0;onlineOtherReserve=0;cpuSpecific=$false;fullProgramCompiled=$false;hardwareQualified=$false;files=@(Get-ChildItem $destination -File|ForEach-Object {@{name=$_.Name;bytes=$_.Length;sha256=(Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()}})}
$provenance|ConvertTo-Json -Depth 6|Set-Content (Join-Path $destination 'provenance.json') -Encoding utf8
'RESULT='+$destination
'CASES='+$inputCases.Count
