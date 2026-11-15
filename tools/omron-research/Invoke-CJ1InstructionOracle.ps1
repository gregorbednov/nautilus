# Research only. Own worker, no GUI actions and no PLC APIs.
[CmdletBinding()]
param(
 [Parameter(Mandatory=$true)][string]$Cases,
 [Parameter(Mandatory=$true)][string]$Output,
 [switch]$ControlProbe,
 [switch]$InvocationProbe,
 [switch]$ParameterProbe,
 [switch]$FBBodyProbe,
 [switch]$DecodeProbe,
	[ValidateSet('library-default','bcd','binary')][string]$TimerCounterMode='library-default',
 [string]$Frida=(Join-Path $PSScriptRoot '..\..\..\tools\frida-17.26.3\frida-inject.exe')
)
$ErrorActionPreference='Stop'
if($DecodeProbe -and $TimerCounterMode -ne 'bcd'){throw 'Decode probe requires an explicit BCD profile'}
if(([int]$ControlProbe.IsPresent+[int]$InvocationProbe.IsPresent+[int]$ParameterProbe.IsPresent+[int]$FBBodyProbe.IsPresent+[int]$DecodeProbe.IsPresent) -gt 1){throw 'Select only one research probe'}
$binary='C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\CX-Pdata.dll'
$compilerHash='a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141'
if((Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant() -ne $compilerHash){throw 'Unreviewed compiler binary'}
$fridaPath=(Resolve-Path -LiteralPath $Frida).Path
if((Get-FileHash -LiteralPath $fridaPath -Algorithm SHA256).Hash -ne 'A51CE388A6DE6C6E6FFF1C624CD439B280AC2EDEFA82162EF3F3F2BF98BD6432'){throw 'Unreviewed injector'}
$casesPath=(Resolve-Path -LiteralPath $Cases).Path
if((Get-Item -LiteralPath $casesPath).Length -gt 1048576){throw 'Case input too large'}
$inputCases=@(Get-Content -LiteralPath $casesPath -Raw | ConvertFrom-Json)
if($inputCases.Count -eq 0 -or $inputCases.Count -gt 512){throw 'Invalid case count'}
$ids=@{}
foreach($case in $inputCases){
 if($DecodeProbe -and ($case.code -isnot [array] -or $case.code.Count -eq 0 -or $case.code.Count -gt 128 -or $case.sourceSHA256 -cnotmatch '^[0-9a-f]{64}$' -or ($case.sourceOffsetWords -isnot [int] -and $case.sourceOffsetWords -isnot [long]) -or $case.sourceOffsetWords -lt 0)){throw 'Invalid captured decode input'}
 if($DecodeProbe){foreach($word in $case.code){if($word -is [bool] -or $word -isnot [ValueType] -or $word -lt 0 -or $word -gt 4294967295 -or [decimal]$word -ne [decimal][uint32]$word){throw 'Invalid instruction container'}}}
 if($ParameterProbe -and ($case.mnemonic -cne 'FBCALL' -or @($case.operands).Count -ne 1 -or $case.operands[0] -cne '&0' -or $case.parameter.type -cnotin @('BOOL','WORD','UINT','INT') -or $case.parameter.usage -notin @(1,2) -or $case.parameter.source -isnot [string] -or $case.parameter.source -cnotmatch '^(#[0-9A-F]{1,4}|&[0-9]{1,5}|(CIO|W|H|D)?[0-9]{1,5}(\.[0-9]{1,2})?)$')){throw 'Invalid scalar parameter probe case'}
 if($InvocationProbe -and ($case.mnemonic -cne 'FBCALL' -or $case.invocationOption -isnot [bool] -or @($case.operands).Count -ne 1 -or $case.operands[0] -cne '&0')){throw 'Invocation probe requires FBCALL &0 and an explicit boolean invocationOption'}
 if(-not $case.id -or $ids.ContainsKey($case.id) -or $case.mnemonic -notmatch '^[A-Za-z@%][A-Za-z0-9@%()]*$' -or $case.operands -isnot [array] -or @($case.operands).Count -gt 8){throw 'Invalid/duplicate instruction case'}
 $ids[$case.id]=$true
 foreach($operand in $case.operands){if($operand -isnot [string] -or $operand.Length -gt 128 -or $operand.Contains([char]0)){throw 'Invalid operand input'}}
}
$destination=[IO.Path]::GetFullPath($Output)
if(Test-Path -LiteralPath $destination){throw 'Output directory must be new'}
$null=New-Item -ItemType Directory -Path $destination
Copy-Item -LiteralPath $casesPath -Destination (Join-Path $destination 'cases.json')
$template='instruction-oracle.js'
if($DecodeProbe){$template='decode-oracle.js'}
Copy-Item -LiteralPath (Join-Path $PSScriptRoot $template) -Destination (Join-Path $destination 'oracle-source.js')
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'instruction-worker.ps1') -Destination (Join-Path $destination 'worker.ps1')
$request=[ordered]@{binary=$binary;compilerSHA256=$compilerHash;requestSHA256=(Get-FileHash -LiteralPath $casesPath -Algorithm SHA256).Hash.ToLowerInvariant();timerCounterMode=$TimerCounterMode;profiles=@(@{device='CJ1M';cpu='CPU21'},@{device='CJ1H-H';cpu='CPU65'});cases=$inputCases}
if($ControlProbe){$request.probe='control-bits-v1'}
if($InvocationProbe){$request.probe='invocation-skeleton-v1'}
if($ParameterProbe){$request.probe='parameter-transfer-v1'}
if($FBBodyProbe){$request.probe='fb-body-operands-v1';$request.programType=32}
if($DecodeProbe){$request.probe='fb-native-decode-v1';$request.programType=32}
$script='const request = '+($request | ConvertTo-Json -Compress -Depth 8)+";`n"+(Get-Content -LiteralPath (Join-Path $destination 'oracle-source.js') -Raw)
$scriptPath=Join-Path $destination 'oracle.js'
[IO.File]::WriteAllText($scriptPath,$script,(New-Object Text.UTF8Encoding($false)))
$worker=$null;$injector=$null
try{
 $worker=Start-Process -FilePath 'C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe' -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-File',('"'+(Join-Path $destination 'worker.ps1')+'"'),'-Directory',('"'+$destination+'"')) -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $destination 'worker.log') -RedirectStandardError (Join-Path $destination 'worker.stderr')
 for($iteration=0;$iteration -lt 100;$iteration++){if(Test-Path -LiteralPath (Join-Path $destination 'worker-ready.txt')){break};if($worker.HasExited){throw 'Worker exited early'};Start-Sleep -Milliseconds 100}
 if(-not (Test-Path -LiteralPath (Join-Path $destination 'worker-ready.txt'))){throw 'Worker ready timeout'}
 $injector=Start-Process -FilePath $fridaPath -ArgumentList @('-p',[string]$worker.Id,'-s',('"'+$scriptPath+'"'),'-R','qjs') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $destination 'events.log') -RedirectStandardError (Join-Path $destination 'injector.stderr')
 $finished=$false
 for($iteration=0;$iteration -lt 300;$iteration++){
  if((Get-Content -LiteralPath (Join-Path $destination 'events.log') -Raw -ErrorAction SilentlyContinue) -match '"kind":"(done|error)"'){$finished=$true;break}
  if($worker.HasExited -or $injector.HasExited){break};Start-Sleep -Milliseconds 100
 }
 if(-not $finished){throw 'Library oracle did not complete within deadline'}
}finally{
 if($injector -and -not $injector.HasExited){Stop-Process -Id $injector.Id;$injector.WaitForExit()}
 if($worker -and -not $worker.HasExited){[IO.File]::WriteAllText((Join-Path $destination 'worker-stop.txt'),'stop');if(-not $worker.WaitForExit(2000)){Stop-Process -Id $worker.Id}}
}
$events=@(Get-Content -LiteralPath (Join-Path $destination 'events.log') | Where-Object {$_.StartsWith('CJ1_INSTRUCTION ')} | ForEach-Object {$_.Substring(16) | ConvertFrom-Json})
if(@($events | Where-Object kind -eq 'error').Count -ne 0 -or @($events | Where-Object kind -eq 'case').Count -ne 2*$inputCases.Count -or @($events | Where-Object kind -eq 'done').Count -ne 1){throw 'Library oracle returned incomplete/error results'}
foreach($name in @('worker.stderr','injector.stderr')){if((Get-Item -LiteralPath (Join-Path $destination $name)).Length -ne 0){throw ('Oracle stderr is not empty: '+$name)}}
$provenance=[ordered]@{schemaVersion=1;method='native-library-calls-in-disposable-worker';compilerSHA256=$compilerHash;requestSHA256=$request.requestSHA256;programType=0;unitVersion=$null;timerCounterMode=$TimerCounterMode;fullProgramCompiled=$false;hardwareQualified=$false;files=@(Get-ChildItem -LiteralPath $destination -File | Where-Object Name -in @('cases.json','events.log','oracle.js','oracle-source.js','worker.ps1','worker.stderr','injector.stderr') | ForEach-Object {@{name=$_.Name;sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant();bytes=$_.Length}})}
if($ControlProbe){$provenance.probe='control-bits-v1'}
if($InvocationProbe){$provenance.probe='invocation-skeleton-v1'}
if($ParameterProbe){$provenance.probe='parameter-transfer-v1'}
if($FBBodyProbe){$provenance.probe='fb-body-operands-v1';$provenance.programType=32}
if($DecodeProbe){$provenance.probe='fb-native-decode-v1';$provenance.programType=32}
$provenance | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $destination 'provenance.json') -Encoding utf8
Write-Output ('RESULT='+$destination)
Write-Output ('CASES='+(2*$inputCases.Count))
