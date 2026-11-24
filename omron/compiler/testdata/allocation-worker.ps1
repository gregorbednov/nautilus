param([Parameter(Mandatory=$true)][string]$Directory)
$ErrorActionPreference='Stop'
if([IntPtr]::Size -ne 4){throw 'Allocation oracle requires x86 PowerShell'}
$binary='C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\PHeadDoc.dll'
$expected='463eb4402bb6fa00453a5d7a745d77ced6be454d77aa2a683eb67de7e38ba71c'
function FileSHA([string]$path){$hash=[Security.Cryptography.SHA256]::Create();try{([BitConverter]::ToString($hash.ComputeHash([IO.File]::ReadAllBytes($path)))).Replace('-','').ToLowerInvariant()}finally{$hash.Dispose()}}
if((FileSHA $binary) -cne $expected){throw 'Unreviewed PHeadDoc binary'}
$casesPath=Join-Path $Directory 'cases.json'
$cases=Get-Content -LiteralPath $casesPath -Raw|ConvertFrom-Json
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Runtime.InteropServices.ComTypes;
using TYPEATTR = System.Runtime.InteropServices.ComTypes.TYPEATTR;
public static class CJ1OfflineAllocationEvents {
 [DllImport("oleaut32.dll",CharSet=CharSet.Unicode,PreserveSig=false)]
 static extern void LoadTypeLibEx(string file,int kind,out ITypeLib library);
 public static Guid IID;
 static Func<int,int> boolSize,wordSize;
 public static List<int> Requests=new List<int>();
 public static void Initialize(string file) {
  ITypeLib library;LoadTypeLibEx(file,2,out library);
  try {
   for(int i=0;i<library.GetTypeInfoCount();i++) {
    ITypeInfo info;library.GetTypeInfo(i,out info);
    try {
     string name,doc,help;int context;info.GetDocumentation(-1,out name,out doc,out context,out help);
     if(name=="_ICXP_POUHeadDocEvents2") {IntPtr pointer;info.GetTypeAttr(out pointer);try{IID=((TYPEATTR)Marshal.PtrToStructure(pointer,typeof(TYPEATTR))).guid;}finally{info.ReleaseTypeAttr(pointer);}}
    }finally{Marshal.ReleaseComObject(info);}
   }
  }finally{Marshal.ReleaseComObject(library);}
  if(IID==Guid.Empty)throw new Exception("Missing POU event interface");
  // Offline owner: there is no online BOOL/OTHER memory reservation.
  boolSize=b=>{Requests.Add(b);return 0;};wordSize=b=>{Requests.Add(b);return 0;};
 }
 public static void Attach(object document) {Requests.Clear();ComEventsHelper.Combine(document,IID,10,boolSize);ComEventsHelper.Combine(document,IID,11,wordSize);}
 public static void Detach(object document) {ComEventsHelper.Remove(document,IID,10,boolSize);ComEventsHelper.Remove(document,IID,11,wordSize);}
}
'@
[CJ1OfflineAllocationEvents]::Initialize($binary)
$script:sequence=0
function Emit([string]$kind,$fields){$event=[ordered]@{schemaVersion=1;kind=$kind;sequence=$script:sequence};$script:sequence++;foreach($key in $fields.Keys){$event[$key]=$fields[$key]};'CJ1_ALLOCATION '+($event|ConvertTo-Json -Depth 10 -Compress)}
Emit 'begin' ([ordered]@{method='com-pou-header-offline-allocation';pheadSHA256=$expected;requestSHA256=(FileSHA $casesPath);pointerBytes=4;progID='Omron.CXP.POUHeadDoc.Doc2';eventIID=[CJ1OfflineAllocationEvents]::IID.ToString();onlineBoolReserve=0;onlineOtherReserve=0;cpuSpecific=$false;fullProgramCompiled=$false;hardwareQualified=$false})
try {
 foreach($case in $cases) {
  $doc=$null
  try {
   $doc=New-Object -ComObject Omron.CXP.POUHeadDoc.Doc2
   [CJ1OfflineAllocationEvents]::Attach($doc)
   $doc.SetFBOnLineEditMode(0)
   if($doc.IsFBOnlineEditing() -ne 0){throw 'Online edit mode readback mismatch'}
   $module=[Diagnostics.Process]::GetCurrentProcess().Modules|Where-Object {$_.ModuleName -ieq 'PHeadDoc.dll'}
   $loadedHash=FileSHA $module.FileName
   if($loadedHash -cne $expected){throw 'Loaded PHeadDoc differs from reviewed binary'}
   $dispatch=[Runtime.InteropServices.Marshal]::GetIDispatchForObject($doc)
   $apiRVAs=[ordered]@{}
   try {
    $table=[Runtime.InteropServices.Marshal]::ReadIntPtr($dispatch)
    foreach($api in @(@('getAddressOffset',56),@('getMemorySize',60),@('getUsedBOOLAndWORDSize',388))){$pointer=[Runtime.InteropServices.Marshal]::ReadIntPtr($table,[int]$api[1]);$apiRVAs[$api[0]]=($pointer.ToInt64()-$module.BaseAddress.ToInt64()).ToString('x')}
   }finally{[void][Runtime.InteropServices.Marshal]::Release($dispatch)}
   if($doc.GetItemCount() -ne 0){throw 'New POU is not empty'}
   foreach($variable in $case.variables){$doc.AddVariable([int]$variable.usage,[string]$variable.name,[int]$variable.typeCode,0,'',[string]$variable.initial,'',0,0)}
   if($doc.GetItemCount() -ne $case.variables.Count){throw 'Declaration count mismatch'}
   $memory=@();foreach($base in 0..4){$memory+=@{baseRegister=$base;sizeBits=$doc.GetMemorySize($base)}}
   $packed=@();foreach($base in @(3,4)){$boolWords=0;$otherWords=0;$doc.GetUsedBOOLAndWORDSize($base,[ref]$boolWords,[ref]$otherWords);$packed+=@{baseRegister=$base;boolWords=$boolWords;otherWords=$otherWords}}
   $bindings=@()
   foreach($name in @('EN','ENO')+@($case.variables|ForEach-Object {$_.name})) {
    $variable=[string]$name;$offset=[string]''
    $result=$doc.GetAddressOffset([ref]$variable,[ref]$offset)
    if($result -ne 1 -or $variable -cne $name -or $offset -cnotmatch '^[0-9]+,BR[0-4]$'){throw 'Invalid allocation result'}
    $bindings+=@{name=$name;reference=$offset}
   }
   Emit 'case' ([ordered]@{id=$case.id;loadedPheadSHA256=$loadedHash;apiRVAs=$apiRVAs;onlineEditing=[bool]$doc.IsFBOnlineEditing();variables=@($case.variables);bindings=$bindings;memory=$memory;packed=$packed;eventRequests=@([CJ1OfflineAllocationEvents]::Requests.ToArray())})
  }finally{if($doc){[CJ1OfflineAllocationEvents]::Detach($doc);[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($doc)}}
 }
 Emit 'done' @{caseCount=$cases.Count}
}catch{Emit 'error' @{message=$_.Exception.Message};exit 1}
