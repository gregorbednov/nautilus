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
  const setCII=native(0x175eb3,'void',['pointer','pointer','int']);
  const generateInvocation=native(0x171499,'int',['pointer','pointer','pointer','int']);
  const finishInvocation=native(0x17166f,'int',['pointer','pointer']);
  const stringToType=native(0x175f7c,'uint32',['pointer'],'mscdecl');
  const operandPreamble=native(0x1749ea,'void',['pointer','pointer','pointer','pointer','int','int','uint32','int','pointer','int','int']);
  const getBase=native(0x1ce6a1,'int',['pointer','int','pointer','pointer']);
  const getBaseAddress=native(0x1d0137,'int',['pointer','int','pointer','pointer']);
  const setUseFB=native(0x1cf742,'void',['pointer','int']);
  const getUseFB=native(0x1cf79a,'int',['pointer']);
  if(request.probe!==undefined && !['control-bits-v1','invocation-skeleton-v1','parameter-transfer-v1','fb-body-operands-v1','fb-body-context-v1','fb-instruction-compile-v1','fb-definition-properties-v1','fb-instance-compile-v1','fb-section-addresses-v1','task-properties-v1'].includes(request.probe))throw new Error('unsupported probe');
  const programType=request.programType===undefined?0:request.programType;
  if(programType!==(request.probe==='fb-instance-compile-v1'?16:['fb-body-operands-v1','fb-body-context-v1','fb-instruction-compile-v1','fb-definition-properties-v1'].includes(request.probe)?32:0))throw new Error('invalid program type/probe combination');
  const compileContext=['fb-body-context-v1','fb-instruction-compile-v1','fb-definition-properties-v1','fb-instance-compile-v1'].includes(request.probe);
  const useCompile=['fb-instruction-compile-v1','fb-definition-properties-v1','fb-instance-compile-v1'].includes(request.probe);
  let arrayCtor,arrayDtor,arraySet,compileInstruction,isCompiled;
  if(useCompile){
   const mfc=Process.enumerateModules().find(x=>x.name.toLowerCase()==='mfc42.dll');if(!mfc)throw new Error('missing MFC42');
   const getProc=new NativeFunction(Process.getModuleByName('kernel32.dll').getExportByName('GetProcAddress'),'pointer',['pointer','pointer'],'stdcall');
   const ctorThunk=library.base.add(0x24e332);
   if(ctorThunk.readU8()!==0xff || ctorThunk.add(1).readU8()!==0x25 || !ctorThunk.add(2).readPointer().readPointer().equals(getProc(mfc.base,ptr(541))))throw new Error('unreviewed CStringArray constructor import');
   function ordinal(number,result,args){const address=getProc(mfc.base,ptr(number));if(address.isNull())throw new Error('missing reviewed MFC ordinal');return new NativeFunction(address,result,args,'thiscall');}
   arrayCtor=ordinal(541,'pointer',['pointer']);arrayDtor=ordinal(801,'void',['pointer']);arraySet=ordinal(5861,'void',['pointer','int','pointer']);
   compileInstruction=native(0x16b432,'int',['pointer','pointer','int','pointer','pointer','int','int','int','int','int','int']);
   isCompiled=native(0x16b3e3,'int',['pointer']);
  }
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
  emit('begin',{compilerSHA256:request.compilerSHA256,requestSHA256:request.requestSHA256,fridaVersion:Frida.version,architecture:Process.arch,programType,unitVersion:null,timerCounterMode:timerMode,fullProgramCompiled:false,probe:request.probe});
  initLanguage();
  const kbase=Memory.alloc(0x4000);retained.push(kbase);kbaseCtor(kbase);
  const profileStrings=[];
  let count=0;
  try{
   for(const cpu of request.profiles){
    const selected=setPLC(kbase,string(cpu.device,profileStrings),string(cpu.cpu,profileStrings));
    if(selected!==1)throw new Error('SetPLC failed for '+cpu.device+' '+cpu.cpu);
    if(compileContext){setUseFB(kbase,1);if(getUseFB(kbase)!==1)throw new Error('FB mode readback mismatch');}
    if(timerMode==='bcd')setBCD(kbase);
    if(timerMode==='binary')setBinary(kbase);
    const bcd=isBCD(kbase),binary=isBinary(kbase);
    if((bcd!==0 && bcd!==1) || binary!==1-bcd || (timerMode==='bcd' && bcd!==1) || (timerMode==='binary' && binary!==1))throw new Error('timer mode readback mismatch');
    const descriptor=kbase.add(0x2d50).readPointer();
    const profile={device:cpu.device,cpu:cpu.cpu,selector:descriptor.add(0x44).readU32(),familyMaskLow:descriptor.add(0x68).readU32(),familyMaskHigh:descriptor.add(0x6c).readU32(),timerModeBcd:bcd,timerModeBinary:binary};
    if(compileContext)profile.useFunctionBlocks=getUseFB(kbase);
    if(['fb-body-operands-v1','fb-body-context-v1','fb-instruction-compile-v1','fb-definition-properties-v1','fb-instance-compile-v1'].includes(request.probe)){
     profile.bases=[];
     for(let index=0;index<5;index++){
      const name=string('',profileStrings),size=Memory.alloc(4),address=Memory.alloc(4),secondSize=Memory.alloc(4);
      if(getBase(kbase,index,name,size)!==1 || getBaseAddress(kbase,index,address,secondSize)!==1 || size.readS32()!==secondSize.readS32())throw new Error('base register readback failed');
      profile.bases.push({index,name:text(name.readPointer()),address:address.readS32(),size:size.readS32()});
     }
    }
    emit('profile',profile);
    for(const test of request.cases){
     const instance=Memory.alloc(0x100);retained.push(instance);instructionCtor(instance);
     const owned=[];
     const result={id:test.id,device:cpu.device,cpu:cpu.cpu,mnemonic:test.mnemonic,operands:test.operands,initialiseResult:null,operandResults:[],normalized:[],encodeResult:null,words:[],accepted:false};
     try{
      if(request.probe==='task-properties-v1'){
       const p=test.taskProperties;if(!p || !Number.isInteger(p.number) || p.number < -2 || p.number > 65535 || ![0,1].includes(p.protection) || !/^[A-Za-z0-9_]{0,25}$/.test(p.name))throw new Error('invalid task property case');
       const size=library.base.add(0x2726bc).readU32();if(size!==752)throw new Error('unreviewed program size');
       const program=Memory.alloc(size);retained.push(program);native(0x1d5677,'pointer',['pointer'])(program);
       try{
        native(0x1d6216,'void',['pointer','int'])(program,0);
        const initialNumber=native(0x12ce0,'int',['pointer'])(program);
        const numberResult=native(0x215f70,'int',['pointer','pointer','int'])(program,kbase,p.number);
        native(0x12d80,'void',['pointer','pointer'])(program,string(p.name,owned));
        const protectionResult=native(0x21602c,'int',['pointer','pointer','int'])(program,kbase,p.protection);
        const name=string('',owned);native(0x12da0,'void',['pointer','pointer'])(program,name);
        const number=native(0x12ce0,'int',['pointer'])(program);
        const taskType=number>=0 && number<288?native(0x1ca1ee,'int',['pointer','int'])(kbase,number):null;
        result.taskProperties={initialNumber,numberResult,number,name:text(name.readPointer()),protectionResult,protection:native(0x12ca0,'int',['pointer'])(program),taskType,programType:native(0x12e40,'int',['pointer'])(program),recordFlags:program.add(0x200).readU32(),descriptorTaskCount:descriptor.add(0x58).readS32()};
       }finally{native(0x1d5c77,'void',['pointer'])(program);}
      }
      if(request.probe==='fb-section-addresses-v1'){
       const p=test.nextEnable;if(!p || !Number.isInteger(p.index) || p.index < -1 || p.index > 7 || !Number.isInteger(p.value) || p.value < -2147483648 || p.value > 2147483647)throw new Error('invalid next-enable case');
       const setNext=native(0x1cf6e3,'int',['pointer','int','int']);
       const readNext=()=>{const a=[];for(let i=0;i<7;i++)a.push(kbase.add(0x2d7c+i*4).readU32());return a;};
       const before=readNext();
       const returned=setNext(kbase,p.index,p.value),after=readNext();
       for(let i=0;i<7;i++)if(setNext(kbase,i,0)!==1)throw new Error('reset through setter failed');
       result.nextEnable={index:p.index,value:p.value,returned,before,after,cleared:readNext()};
      }
      if(request.probe==='fb-definition-properties-v1'){
       const p=test.definitionProperties;if(!p || !Number.isInteger(p.number) || !Number.isInteger(p.inputCount) || !Number.isInteger(p.outputCount))throw new Error('invalid definition properties');
       const size=library.base.add(0x2726bc).readU32();if(size!==752)throw new Error('unreviewed program size');
       const program=Memory.alloc(size);retained.push(program);
       native(0x1d5677,'pointer',['pointer'])(program);
       try{
        native(0x1d6216,'void',['pointer','int'])(program,32);
        native(0x12e60,'void',['pointer','int'])(program,p.number);
        native(0x12d80,'void',['pointer','pointer'])(program,string(p.name,owned));
        const inputs=native(0x21b323,'int',['pointer','int'])(program,p.inputCount),outputs=native(0x21b34b,'int',['pointer','int'])(program,p.outputCount);
        const name=string('',owned);native(0x12da0,'void',['pointer','pointer'])(program,name);
        result.properties={number:native(0x12e80,'int',['pointer'])(program),name:text(name.readPointer()),inputsResult:inputs,outputsResult:outputs,inputCount:program.add(0xec).readS32(),outputCount:program.add(0xf0).readS32(),flags:program.add(0x200).readU32()};
       }finally{native(0x1d5c77,'void',['pointer'])(program);}
      }
      result.initialiseResult=initialise(instance,kbase,string(test.mnemonic,owned),0);
      const definition=instance.add(0x2c).readPointer();
      if(result.initialiseResult===1 && !definition.isNull()){
       result.definition=text(definition.add(12).readPointer());
       result.arity=definition.add(0xc0).readU8();
       if(result.arity===test.operands.length){
        if(useCompile){
         const array=Memory.alloc(20);retained.push(array);arrayCtor(array);
         try{
          for(let i=0;i<test.operands.length;i++){const chars=Memory.allocUtf8String(test.operands[i]);retained.push(chars);arraySet(array,i,chars);}
          // Same explicit argument values as PLC_Program::GenerateInst.
          result.compileResult=compileInstruction(instance,array,0,ptr(0),ptr(0),0,0,1,0,1,programType);
          result.compiledFlag=isCompiled(instance);
          const state=snapshot(instance);result.words=state.words;result.flags=state.flags;
          for(let i=0;i<result.arity;i++){const record=instance.add(0x10).readPointer().add(i*0x30);result.operandResults.push(record.readU32());result.normalized.push({value:record.add(4).readU32(),flags:record.add(8).readU32()});}
          result.accepted=result.compileResult===1 && result.compiledFlag!==0 && result.words.length>0;
          if(result.accepted && request.probe==='fb-instance-compile-v1'){
           setCII(instance,kbase,1);result.instanceStages=[Object.assign({operation:'SetCIIFlag(true)'},snapshot(instance))];
           if(test.mnemonic==='FBBCALL'){setCC(instance);result.instanceStages.push(Object.assign({operation:'SetCCFlags'},snapshot(instance)));}
          }
          if(!result.accepted)result.words=[];
         }finally{arrayDtor(array);}
        }else{
        for(let index=0;index<test.operands.length;index++){
         const validation=operandEncode(definition,kbase,instance,string(test.operands[index],owned),index,programType);
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
