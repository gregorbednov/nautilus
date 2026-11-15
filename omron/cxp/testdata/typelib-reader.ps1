# Research-only metadata reader. Does not instantiate vendor COM objects.
[CmdletBinding()]
param(
 [Parameter(Mandatory=$true)][string]$Binary,
 [Parameter(Mandatory=$true)][string]$ExpectedSHA256,
 [Parameter(Mandatory=$true)][string]$Output
)
$ErrorActionPreference='Stop'
$binaryPath=(Resolve-Path -LiteralPath $Binary).Path
$hash=(Get-FileHash -LiteralPath $binaryPath -Algorithm SHA256).Hash.ToLowerInvariant()
if($hash -cne $ExpectedSHA256.ToLowerInvariant()){throw 'Unreviewed type library binary'}
$outputPath=[IO.Path]::GetFullPath($Output)
if(Test-Path -LiteralPath $outputPath){throw 'Output must be new'}
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Runtime.InteropServices.ComTypes;
using TYPEATTR = System.Runtime.InteropServices.ComTypes.TYPEATTR;
using TYPEKIND = System.Runtime.InteropServices.ComTypes.TYPEKIND;
using VARDESC = System.Runtime.InteropServices.ComTypes.VARDESC;
using FUNCDESC = System.Runtime.InteropServices.ComTypes.FUNCDESC;
using ELEMDESC = System.Runtime.InteropServices.ComTypes.ELEMDESC;
using VARKIND = System.Runtime.InteropServices.ComTypes.VARKIND;
public static class CJ1TypeLibraryReader {
 [DllImport("oleaut32.dll", CharSet=CharSet.Unicode, PreserveSig=false)]
 static extern void LoadTypeLibEx(string file, int registrationKind, out ITypeLib library);
 public sealed class Member { public string Name; public long Value; }
 public sealed class Enumeration { public string Name; public List<Member> Members = new List<Member>(); }
 public static List<string> Functions(string file) {
  ITypeLib library;LoadTypeLibEx(file,2,out library);
  try {
   var result=new List<string>();int count=library.GetTypeInfoCount();
   if(count>10000) throw new Exception("Type count bound exceeded");
   for(int i=0;i<count;i++) {
    ITypeInfo info;library.GetTypeInfo(i,out info);
    try {
     string owner,doc,help;int context;info.GetDocumentation(-1,out owner,out doc,out context,out help);
     IntPtr pointer;info.GetTypeAttr(out pointer);TYPEATTR attr;
     try { attr=(TYPEATTR)Marshal.PtrToStructure(pointer,typeof(TYPEATTR)); }finally{info.ReleaseTypeAttr(pointer);}
     if(attr.cFuncs>4096) throw new Exception("Function count bound exceeded");
     for(int j=0;j<attr.cFuncs;j++) {
      IntPtr fn;info.GetFuncDesc(j,out fn);
      try {
       var desc=(FUNCDESC)Marshal.PtrToStructure(fn,typeof(FUNCDESC));
       if(desc.cParams>128) throw new Exception("Parameter count bound exceeded");
       string[] names=new string[129];int actual;info.GetNames(desc.memid,names,names.Length,out actual);
       var parameters=new List<string>();
       for(int k=0;k<desc.cParams;k++) {
        var element=(ELEMDESC)Marshal.PtrToStructure(IntPtr.Add(desc.lprgelemdescParam,k*Marshal.SizeOf(typeof(ELEMDESC))),typeof(ELEMDESC));
        parameters.Add((k+1<actual?names[k+1]:"?")+":"+element.tdesc.vt);
       }
       result.Add(owner+"."+names[0]+"("+String.Join(",",parameters)+"):"+desc.elemdescFunc.tdesc.vt+";vtable="+desc.oVft+";member="+desc.memid);
      }finally{info.ReleaseFuncDesc(fn);}
     }
    }finally{Marshal.ReleaseComObject(info);}
   }
   return result;
  }finally{Marshal.ReleaseComObject(library);}
 }
 public static List<string> TypeNames(string file) {
  ITypeLib library;LoadTypeLibEx(file,2,out library);
  try {
   int count=library.GetTypeInfoCount();if(count>10000) throw new Exception("Type count bound exceeded");
   var result=new List<string>();
   for(int i=0;i<count;i++) { string name,doc,help;int context;TYPEKIND kind;
    library.GetDocumentation(i,out name,out doc,out context,out help);library.GetTypeInfoType(i,out kind);
    result.Add(name+":"+kind);
   }
   return result;
  } finally { Marshal.ReleaseComObject(library); }
 }
 public static List<Enumeration> Read(string file) {
  ITypeLib library;
  LoadTypeLibEx(file, 2, out library); // REGKIND_NONE; no registration.
  try {
   int count=library.GetTypeInfoCount();
   if(count<0 || count>10000) throw new Exception("Type count bound exceeded");
   var result=new List<Enumeration>();
   for(int i=0;i<count;i++) {
    TYPEKIND kind; library.GetTypeInfoType(i,out kind);
    if(kind!=TYPEKIND.TKIND_ENUM) continue;
    ITypeInfo info; library.GetTypeInfo(i,out info);
    try {
     IntPtr pointer; info.GetTypeAttr(out pointer);
     TYPEATTR attr;
     try { attr=(TYPEATTR)Marshal.PtrToStructure(pointer,typeof(TYPEATTR)); }
     finally { info.ReleaseTypeAttr(pointer); }
     if(attr.cVars<0 || attr.cVars>4096) throw new Exception("Enum size bound exceeded");
     string name,doc,help;int context;
     info.GetDocumentation(-1,out name,out doc,out context,out help);
     var enumeration=new Enumeration();enumeration.Name=name;
     for(int j=0;j<attr.cVars;j++) {
      IntPtr variable;info.GetVarDesc(j,out variable);
      try {
       var desc=(VARDESC)Marshal.PtrToStructure(variable,typeof(VARDESC));
       if(desc.varkind!=VARKIND.VAR_CONST) throw new Exception("Enum member is not constant");
       info.GetDocumentation(desc.memid,out name,out doc,out context,out help);
       enumeration.Members.Add(new Member {Name=name,Value=Convert.ToInt64(Marshal.GetObjectForNativeVariant(desc.desc.lpvarValue))});
      } finally { info.ReleaseVarDesc(variable); }
     }
     result.Add(enumeration);
    } finally { Marshal.ReleaseComObject(info); }
   }
   return result;
  } finally { Marshal.ReleaseComObject(library); }
 }
}
'@
$enums=[CJ1TypeLibraryReader]::Read($binaryPath)
$result=[ordered]@{schemaVersion=1;method='oleaut32-LoadTypeLibEx-REGKIND_NONE';sourceSHA256=$hash;sourceName=[IO.Path]::GetFileName($binaryPath);pointerBytes=[IntPtr]::Size;vendorObjectCreated=$false;typeNames=@([CJ1TypeLibraryReader]::TypeNames($binaryPath));functions=@([CJ1TypeLibraryReader]::Functions($binaryPath));enums=@($enums)}
$result | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $outputPath -Encoding utf8
Write-Output ('RESULT='+$outputPath)
