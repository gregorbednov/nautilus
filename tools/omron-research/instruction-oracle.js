'use strict';
// Research-only calls to a hash-pinned library in a disposable worker.
// `request` is prepended as JSON by Invoke-CJ1InstructionOracle.ps1.
let sequence=0;
function emit(kind,fields){console.log('CJ1_INSTRUCTION '+JSON.stringify(Object.assign({schemaVersion:1,kind,sequence:sequence++},fields)));}
function text(address){let value='';for(let i=0;i<256;i++){const n=address.add(i).readU8();if(!n)return value;value+=String.fromCharCode(n);}throw new Error('string bound exceeded');}
setImmediate(()=>{
 try{
  if(Process.arch!=='ia32' || request.cases.length>512)throw new Error('unsupported architecture/case count');
  const library=Module.load(request.binary);
  library.ensureInitialized();
  const native=(rva,result,args,abi='thiscall')=>new NativeFunction(library.base.add(rva),result,args,abi);
  const stringCtor=native(0x24e2de,'pointer',['pointer','pointer']);
  const stringDtor=native(0x24e2a2,'void',['pointer']);
  const initLanguage=native(0x1ba0e1,'void',[],'mscdecl');
  const kbaseCtor=native(0x1b96e5,'pointer',['pointer']);
  const kbaseDtor=native(0x1b9cf0,'void',['pointer']);
  const setPLC=native(0x1ba36d,'int',['pointer','pointer','pointer']);
  const setBCD=native(0x1cf6b3,'void',['pointer']);
  const setBinary=native(0x1cf6cb,'void',['pointer']);
  const isBCD=native(0x125e0,'int',['pointer']);
  const isBinary=native(0x12600,'int',['pointer']);
  const timerMode=request.timerCounterMode || 'library-default';
  if(!['library-default','bcd','binary'].includes(timerMode))throw new Error('unsupported timer mode');
  const instructionCtor=native(0x16a25a,'pointer',['pointer']);
  const instructionDtor=native(0x16a375,'void',['pointer']);
  const initialise=native(0x16a9e0,'int',['pointer','pointer','pointer','int']);
  const operandEncode=native(0x32d70,'uint32',['pointer','pointer','pointer','pointer','int','int']);
  const encode=native(0x16d735,'int',['pointer','pointer']);
  const resetCII=native(0x175e8a,'void',['pointer']);
  const setCC=native(0x175e61,'void',['pointer']);
  const generateInvocation=native(0x171499,'int',['pointer','pointer','pointer','int']);
  const finishInvocation=native(0x17166f,'int',['pointer','pointer']);
  const stringToType=native(0x175f7c,'uint32',['pointer'],'mscdecl');
  const operandPreamble=native(0x1749ea,'void',['pointer','pointer','pointer','pointer','int','int','uint32','int','pointer','int','int']);
  if(request.probe!==undefined && !['control-bits-v1','invocation-skeleton-v1','parameter-transfer-v1'].includes(request.probe))throw new Error('unsupported probe');
  function snapshot(instance){
   const count=instance.add(0x34).readU8();if(count>128)throw new Error('control snapshot bound exceeded');
   const buffer=instance.add(0xc).readPointer();if(count && buffer.isNull())throw new Error('missing control buffer');
   const words=[];for(let i=0;i<count;i++)words.push(buffer.add(i*4).readU32());
   return {words,flags:instance.add(0x30).readU32()};
  }
  const retained=[];
  function string(value,owned){const object=Memory.alloc(4);const chars=Memory.allocUtf8String(value);retained.push(object,chars);stringCtor(object,chars);owned.push(object);return object;}
  function readList(instance,offset){
   const list=instance.add(offset),expected=list.add(12).readU32();if(expected>128)throw new Error('invocation list bound exceeded');
   let node=list.add(4).readPointer();const seen=new Set(),instructions=[];
   while(!node.isNull()){
    if(instructions.length>=expected || seen.has(node.toString()))throw new Error('invalid invocation list');seen.add(node.toString());
    const child=node.add(8).readPointer();
    instructions.push(Object.assign({mnemonic:text(child.add(0x38).readPointer())},snapshot(child)));
    node=node.readPointer();
   }
   if(instructions.length!==expected)throw new Error('invocation list count mismatch');return instructions;
  }
  emit('begin',{compilerSHA256:request.compilerSHA256,requestSHA256:request.requestSHA256,fridaVersion:Frida.version,architecture:Process.arch,programType:0,unitVersion:null,timerCounterMode:timerMode,fullProgramCompiled:false,probe:request.probe});
  initLanguage();
  const kbase=Memory.alloc(0x4000);retained.push(kbase);kbaseCtor(kbase);
  const profileStrings=[];
  let count=0;
  try{
   for(const cpu of request.profiles){
    const selected=setPLC(kbase,string(cpu.device,profileStrings),string(cpu.cpu,profileStrings));
    if(selected!==1)throw new Error('SetPLC failed for '+cpu.device+' '+cpu.cpu);
    if(timerMode==='bcd')setBCD(kbase);
    if(timerMode==='binary')setBinary(kbase);
    const bcd=isBCD(kbase),binary=isBinary(kbase);
    if((bcd!==0 && bcd!==1) || binary!==1-bcd || (timerMode==='bcd' && bcd!==1) || (timerMode==='binary' && binary!==1))throw new Error('timer mode readback mismatch');
    const descriptor=kbase.add(0x2d50).readPointer();
    emit('profile',{device:cpu.device,cpu:cpu.cpu,selector:descriptor.add(0x44).readU32(),familyMaskLow:descriptor.add(0x68).readU32(),familyMaskHigh:descriptor.add(0x6c).readU32(),timerModeBcd:bcd,timerModeBinary:binary});
    for(const test of request.cases){
     const instance=Memory.alloc(0x100);retained.push(instance);instructionCtor(instance);
     const owned=[];
     const result={id:test.id,device:cpu.device,cpu:cpu.cpu,mnemonic:test.mnemonic,operands:test.operands,initialiseResult:null,operandResults:[],normalized:[],encodeResult:null,words:[],accepted:false};
     try{
      result.initialiseResult=initialise(instance,kbase,string(test.mnemonic,owned),0);
      const definition=instance.add(0x2c).readPointer();
      if(result.initialiseResult===1 && !definition.isNull()){
       result.definition=text(definition.add(12).readPointer());
       result.arity=definition.add(0xc0).readU8();
       if(result.arity===test.operands.length){
        for(let index=0;index<test.operands.length;index++){
         const validation=operandEncode(definition,kbase,instance,string(test.operands[index],owned),index,0);
         result.operandResults.push(validation);
         const record=instance.add(0x10).readPointer().add(index*0x30);
         record.writeU32(validation); // Same caller contract as EncodeOperands.
         result.normalized.push({value:record.add(4).readU32(),flags:record.add(8).readU32()});
         if(validation!==0)break;
        }
        if(result.operandResults.length===result.arity && result.operandResults.every(code=>code===0)){
         result.encodeResult=encode(instance,kbase);
         const wordCount=instance.add(0x34).readU8();
         if(wordCount>128)throw new Error('instruction word bound exceeded');
         const buffer=instance.add(0x0c).readPointer();
         for(let i=0;i<wordCount;i++)result.words.push(buffer.add(i*4).readU32());
         result.accepted=result.encodeResult===0 && result.words.length>0;
         if(result.accepted && request.probe==='control-bits-v1'){
          result.flagsBeforeControl=instance.add(0x30).readU32();
          resetCII(instance);
          const cleared=snapshot(instance);
          setCC(instance);
          const cc=snapshot(instance);
          result.controlStages=[Object.assign({operation:'ResetCIIflag'},cleared),Object.assign({operation:'SetCCFlags'},cc)];
         }
         if(result.accepted && request.probe==='invocation-skeleton-v1'){
          if(test.mnemonic!=='FBCALL' || test.operands.length!==1 || test.operands[0]!=='&0' || typeof test.invocationOption!=='boolean')throw new Error('invalid invocation probe case');
          // Both CObList members are normally constructed by InstructionInst.
          // These methods use their normal list/compile APIs; no flags or code
          // buffers are injected into the object to bypass native compilation.
          const generated=generateInvocation(instance,kbase,instance.add(0x58),test.invocationOption?1:0);
          const before=snapshot(instance);
          const preamble=readList(instance,0x3c),epilogue=readList(instance,0x58);
          const finished=finishInvocation(instance,kbase);
          result.invocation={option:test.invocationOption,generateResult:generated,finishResult:finished,before,after:snapshot(instance),preamble,epilogue};
         }
         if(result.accepted && request.probe==='parameter-transfer-v1'){
          const p=test.parameter;
          if(test.mnemonic!=='FBCALL' || test.operands.length!==1 || test.operands[0]!=='&0' || !p || !['BOOL','WORD','UINT','INT'].includes(p.type) || ![1,2].includes(p.usage) || typeof p.source!=='string' || !/^(#[0-9A-F]{1,4}|&[0-9]{1,5}|(CIO|W|H|D)?[0-9]{1,5}(\.[0-9]{1,2})?)$/.test(p.source))throw new Error('invalid parameter probe case');
          const dataType=stringToType(string(p.type,owned));if(dataType===0)throw new Error('unrecognized parameter type');
          const source=string(p.source,owned);
          // Scalar count1; unused array/structure dimensions0 and empty formal
          // metadata. The normal method constructs/compiles all child objects.
          operandPreamble(instance,kbase,instance.add(0x3c),source,p.usage,1,dataType,0,string('',owned),0,0);
          result.parameter={input:p,dataType,sourceAfter:text(source.readPointer()),instructions:readList(instance,0x3c)};
         }
        }
       }
      }
     }finally{
      instructionDtor(instance);
      for(const object of owned.reverse())stringDtor(object);
     }
     emit('case',result);count++;
    }
   }
  }finally{
   kbaseDtor(kbase);
   for(const object of profileStrings.reverse())stringDtor(object);
  }
  emit('done',{caseCount:count});
 }catch(error){emit('error',{message:String(error),stack:error.stack});}
});
