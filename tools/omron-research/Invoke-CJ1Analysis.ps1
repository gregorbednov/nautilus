# Research-only. No vendor DLL is loaded or executed by this script.
[CmdletBinding()]
param(
 [Parameter(Mandatory=$true)][string]$GhidraHome,
 [Parameter(Mandatory=$true)][string]$JavaHome,
 [Parameter(Mandatory=$true)][string]$Workspace,
 [Parameter(Mandatory=$true)][string]$Output,
 [string]$Binary='C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\CX-Pdata.dll',
 [string]$ExpectedSHA256='a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141',
 [string[]]$Addresses=@('1216b407','1216b432','121e3eef'),
 [string]$CatalogFacts=''
)
$ErrorActionPreference='Stop'
$binaryPath=(Resolve-Path -LiteralPath $Binary).Path
$hash=(Get-FileHash -LiteralPath $binaryPath -Algorithm SHA256).Hash.ToLowerInvariant()
if($hash -ne $ExpectedSHA256.ToLowerInvariant()){throw 'Binary SHA-256 differs from the reviewed build.'}
if($Addresses.Count -eq 0){throw 'At least one function address is required.'}
foreach($address in $Addresses){if($address -notmatch '^(0x)?[0-9a-fA-F]{8}$'){throw "Invalid address: $address"}}
$workspacePath=[IO.Path]::GetFullPath($Workspace)
# Ghidra rejects project paths containing hidden/dot-prefixed components.
foreach($part in ($workspacePath -split '[\\/]')){if($part.StartsWith('.')){throw 'Ghidra workspace cannot have dot-prefixed path components.'}}
$outputPath=[IO.Path]::GetFullPath($Output)
if(Test-Path -LiteralPath $outputPath){throw "Output already exists: $outputPath"}
if(-not (Test-Path -LiteralPath (Split-Path $outputPath))){throw 'Output parent directory must exist.'}
$headless=Join-Path $GhidraHome 'support\analyzeHeadless.bat'
if(-not (Test-Path -LiteralPath $headless)){throw 'analyzeHeadless.bat was not found.'}
if(-not (Test-Path -LiteralPath (Join-Path $JavaHome 'bin\java.exe'))){throw 'Java runtime was not found.'}
New-Item -ItemType Directory -Force $workspacePath | Out-Null
$copy=Join-Path $workspacePath ($hash+'.dll')
if(Test-Path -LiteralPath $copy){
 if((Get-FileHash -LiteralPath $copy -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hash){throw 'Cached binary hash mismatch.'}
}else{Copy-Item -LiteralPath $binaryPath -Destination $copy}
$projectName='cj1-'+$hash.Substring(0,16)
$argsList=@($workspacePath,$projectName)
if(Test-Path -LiteralPath (Join-Path $workspacePath ($projectName+'.gpr'))){
 $argsList+=@('-process',[IO.Path]::GetFileName($copy),'-readOnly')
}else{$argsList+=@('-import',$copy)}
$argsList+=@('-noanalysis','-max-cpu','1','-scriptPath',(Join-Path $PSScriptRoot 'ghidra'),'-postScript')
if($CatalogFacts){
 $factsPath=(Resolve-Path -LiteralPath $CatalogFacts).Path
 $facts=Get-Content -LiteralPath $factsPath -Raw | ConvertFrom-Json
 if($facts.sha256 -ne $hash -or @($facts.records).Count -ne 1504){throw 'Catalog facts provenance/count mismatch.'}
 $argsList+=@('ExportCJ1Catalog.java',$factsPath,$outputPath)
}else{
 $argsList+=@('ExportCJ1Functions.java',$outputPath)
 $argsList+=$Addresses
}
$argsList+=@('-log',($outputPath+'.log'),'-scriptlog',($outputPath+'.script.log'))
$previousJavaHome=$env:JAVA_HOME
$previousPath=$env:Path
$previousMemory=$env:GHIDRA_HEADLESS_MAXMEM
$previousOptions=$env:GHIDRA_HEADLESS_JAVA_OPTIONS
try {
 $env:JAVA_HOME=$JavaHome
 $env:Path=(Join-Path $JavaHome 'bin')+';'+$env:Path
 $env:GHIDRA_HEADLESS_MAXMEM='768M'
 $env:GHIDRA_HEADLESS_JAVA_OPTIONS='-XX:+UseSerialGC'
 & $headless @argsList
 if($LASTEXITCODE -ne 0){throw "Ghidra failed with exit code $LASTEXITCODE"}
 if(-not (Test-Path -LiteralPath $outputPath)){throw 'Ghidra did not produce the requested result; inspect the log.'}
 $result=Get-Content -LiteralPath $outputPath -Raw | ConvertFrom-Json
 if($result.engine -ne 'ghidra' -or $result.sourceSHA256 -ne $hash){throw 'Output provenance mismatch.'}
 if($CatalogFacts){
  if($result.failedRecords -ne 0 -or @($result.records).Count -ne 1504){throw 'Catalog extraction incomplete; inspect failedRecords and record errors.'}
  if(@($result.records | Where-Object {-not $_.verifiedAgainstPriorFacts}).Count -ne 0){throw 'Unverified catalog records.'}
 }else{
  if(@($result.functions).Count -ne $Addresses.Count){throw 'Output function count mismatch.'}
  foreach($function in $result.functions){if(-not $function.decompileCompleted){throw "Decompilation failed at $($function.requestedAddress): $($function.error)"}}
 }
 Write-Output ('RESULT='+$outputPath)
} finally {
 $env:JAVA_HOME=$previousJavaHome
 $env:Path=$previousPath
 $env:GHIDRA_HEADLESS_MAXMEM=$previousMemory
 $env:GHIDRA_HEADLESS_JAVA_OPTIONS=$previousOptions
}
