# Local research only: observe the pinned x86 compiler, without PLC connections.
[CmdletBinding()]
param(
 [Parameter(Mandatory=$true)][string]$Project,
 [Parameter(Mandatory=$true)][ValidateSet('CJ1M CPU21','CJ1H-H CPU65')][string]$CPU,
 [Parameter(Mandatory=$true)][string]$Output,
 [int]$ProcessId=0,
 [string]$ReplayCapture='',
 [string]$Auditor=(Join-Path $PSScriptRoot '..\..\bin\cj1-oracle.exe'),
 [string]$Frida=(Join-Path $PSScriptRoot '..\..\..\tools\frida-17.26.3\frida-inject.exe')
)
$ErrorActionPreference='Stop'
$auditorPath=(Resolve-Path -LiteralPath $Auditor).Path
$projectPath=(Resolve-Path -LiteralPath $Project).Path
$sourceHash=(Get-FileHash -LiteralPath $projectPath -Algorithm SHA256).Hash.ToLowerInvariant()
$binary='C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\CX-Pdata.dll'
$expectedBinary='a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141'
if($ReplayCapture){
 $replayPath=(Resolve-Path -LiteralPath $ReplayCapture).Path
 $recorded=Get-Content -LiteralPath (Join-Path $replayPath 'result.json') -Raw | ConvertFrom-Json
 if($recorded.sourceSHA256 -ne $sourceHash -or $recorded.declaredCPU -ne $CPU -or $recorded.compilerSHA256 -ne $expectedBinary){throw 'Replay metadata differs from requested source/CPU/compiler'}
}else{
if((Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expectedBinary){throw 'Unreviewed compiler DLL build'}
$fridaPath=(Resolve-Path -LiteralPath $Frida).Path
if((Get-FileHash -LiteralPath $fridaPath -Algorithm SHA256).Hash -ne 'A51CE388A6DE6C6E6FFF1C624CD439B280AC2EDEFA82162EF3F3F2BF98BD6432'){throw 'Unreviewed Frida executable'}
$cxProcesses=@(Get-Process -Name CX-P | Where-Object { $ProcessId -eq 0 -or $_.Id -eq $ProcessId })
if($cxProcesses.Count -ne 1){throw 'Open exactly one CX-Programmer instance before capturing'}
$cxProcess=$cxProcesses[0]
if([IO.Path]::GetFullPath($cxProcess.Path) -ine 'C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\CX-P.exe'){throw 'Unexpected CX-Programmer executable'}
}
$outputPath=[IO.Path]::GetFullPath($Output)
if(Test-Path -LiteralPath $outputPath){throw 'Output must be a new directory'}
$null=New-Item -ItemType Directory -Path $outputPath
$stdout=Join-Path $outputPath 'events.log'
$stderr=Join-Path $outputPath 'stderr.log'
$script=Join-Path $PSScriptRoot 'capture-cj1.js'
$scriptCopy=Join-Path $outputPath 'observer.js'
if($ReplayCapture){
 Copy-Item -LiteralPath (Join-Path $replayPath 'observer.js') -Destination $scriptCopy
 Copy-Item -LiteralPath (Join-Path $replayPath 'events.log') -Destination $stdout
 Copy-Item -LiteralPath (Join-Path $replayPath 'stderr.log') -Destination $stderr
}else{Copy-Item -LiteralPath $script -Destination $scriptCopy}
$observerHash=(Get-FileHash -LiteralPath $scriptCopy -Algorithm SHA256).Hash.ToLowerInvariant()
if($ReplayCapture -and $observerHash -ne $recorded.observerSHA256){throw 'Replay observer hash mismatch'}
if($ReplayCapture -and (Get-FileHash -LiteralPath $stdout -Algorithm SHA256).Hash.ToLowerInvariant() -ne $recorded.eventsSHA256){throw 'Replay event hash mismatch'}
$captureProcess=$null
if(-not $ReplayCapture){
try {
 $captureProcess=Start-Process -FilePath $fridaPath -ArgumentList @('-p', [string]$cxProcess.Id, '-s', ('"'+$scriptCopy+'"'), '-R', 'qjs') -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
 Write-Output 'Capture active for 180 seconds. In CX-Programmer select the intended PLC and press F7.'
 Write-Output ('OUTPUT='+$outputPath)
 # The script detaches all hooks after 180 s. Only our injector is stopped here.
 for($iteration=0;$iteration -lt 37;$iteration++){
  if($captureProcess.WaitForExit(5000)){break}
 }
} finally {
 if($captureProcess -and -not $captureProcess.HasExited){
  Stop-Process -Id $captureProcess.Id -ErrorAction SilentlyContinue
  $captureProcess.WaitForExit()
 }
}
}
$auditPath=Join-Path $outputPath 'capture-audit.json'
$auditErrors=Join-Path $outputPath 'capture-audit-stderr.log'
$auditProcess=Start-Process -FilePath $auditorPath -ArgumentList @('audit-capture','-log',('"'+$stdout+'"'),'-stderr',('"'+$stderr+'"')) -WindowStyle Hidden -RedirectStandardOutput $auditPath -RedirectStandardError $auditErrors -Wait -PassThru
$auditSucceeded=($auditProcess.ExitCode -eq 0)
$events=@(Get-Content -LiteralPath $stdout | Where-Object { $_.StartsWith('CJ1_ORACLE ') } | ForEach-Object { $_.Substring(11) | ConvertFrom-Json })
$ended=@($events | Where-Object kind -eq 'capture-ended')
$instructions=@($events | Where-Object kind -eq 'instruction')
$programs=@($events | Where-Object kind -eq 'program')
$checks=@($events | Where-Object kind -eq 'check-instruction')
$serialized=@($events | Where-Object kind -eq 'serialized-instruction')
$files=@($events | Where-Object kind -eq 'compiler-file')
$compileProfiles=@($events | Where-Object kind -eq 'compile-enter' | ForEach-Object { $_.profile | ConvertTo-Json -Compress } | Sort-Object -Unique)
$errors=@($events | Where-Object kind -eq 'capture-error')
$status='no-compiler-events'
if($checks.Count -gt 0){$status='source-check-observed'}
if($instructions.Count -gt 0){$status='instruction-buffers-observed'}
if($programs.Count -gt 0){$status='program-buffers-observed'}
if($files.Count -gt 0){$status='compiler-file-observed'}
if(-not $auditSucceeded -or $errors.Count -gt 0 -or $ended.Count -ne 1 -or ($ended.Count -eq 1 -and $ended[0].droppedEvents -ne 0)){$status='incomplete-capture'}
$afterHash=(Get-FileHash -LiteralPath $projectPath -Algorithm SHA256).Hash.ToLowerInvariant()
if($afterHash -ne $sourceHash){$status='incomplete-capture'}
$fileResults=@(foreach($fileEvent in $files){
 if(-not $auditSucceeded){continue}
 $fileHex=[string]$fileEvent.fileHex
 if($fileEvent.byteLength -gt 4194304 -or $fileHex.Length -ne 2*$fileEvent.byteLength -or $fileHex -notmatch '^(?:[0-9a-f]{2})*$'){throw 'Invalid compiler file event'}
 $fileBytes=New-Object byte[] $fileEvent.byteLength
 for($fileIndex=0;$fileIndex -lt $fileBytes.Length;$fileIndex++){$fileBytes[$fileIndex]=[Convert]::ToByte($fileHex.Substring($fileIndex*2,2),16)}
 $fileName='compiler-output-'+([int]$fileEvent.sequence)+'.bin'
 $filePath=Join-Path $outputPath $fileName
 [IO.File]::WriteAllBytes($filePath,$fileBytes)
 [ordered]@{file=$fileName; byteLength=$fileBytes.Length; sha256=(Get-FileHash -LiteralPath $filePath -Algorithm SHA256).Hash.ToLowerInvariant(); observedPath=$fileEvent.path}
})
$result=[ordered]@{
 schemaVersion=1; status=$status; method='Frida observer; no native calls or argument replacement';
 replayedFrom=$ReplayCapture;
 declaredCPU=$CPU; cpuDeclarationVerified=$false;
 project=$projectPath; sourceSHA256=$sourceHash; sourceUnchanged=($afterHash -eq $sourceHash);
 sourceIdentityInCompilerVerified=$false; compilerSHA256=$expectedBinary;
 observerSHA256=$observerHash;
 captureVerified=$auditSucceeded;
 auditorSHA256=(Get-FileHash -LiteralPath $auditorPath -Algorithm SHA256).Hash.ToLowerInvariant();
 eventsSHA256=(Get-FileHash -LiteralPath $stdout -Algorithm SHA256).Hash.ToLowerInvariant();
 eventCount=$events.Count; instructionCount=$instructions.Count; programCount=$programs.Count; checkCount=$checks.Count; serializedInstructionCount=$serialized.Count;
 errors=$errors; profiles=@($events | Where-Object profile | ForEach-Object { $_.profile | ConvertTo-Json -Compress } | Sort-Object -Unique);
 compileProfiles=$compileProfiles;
 compilerFiles=$fileResults; compilerBinaryCaptured=($auditSucceeded -and $files.Count -gt 0);
 executableUMCaptured=$false; hardwareQualified=$false;
 limitations=@('CPU and source identity require matching manual compile evidence.',
 'Container bytes are x86 memory order; PLC wire serialization is unverified.',
 'Program buffers alone do not establish a complete serialized UM image.')
}
$result | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $outputPath 'result.json') -Encoding utf8
Write-Output ('STATUS='+$status)
if(-not $result.sourceUnchanged){throw 'Source file changed during capture; reject this comparison'}
if(-not $auditSucceeded){throw 'Capture audit failed; inspect capture-audit.json and raw logs'}
