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
  const instructionCtor=native(0x16a25a,'pointer',['pointer']);
  const instructionDtor=native(0x16a375,'void',['pointer']);
  const initialise=native(0x16a9e0,'int',['pointer','pointer','pointer','int']);
  const operandEncode=native(0x32d70,'uint32',['pointer','pointer','pointer','pointer','int','int']);
  const encode=native(0x16d735,'int',['pointer','pointer']);
  const retained=[];
  function string(value,owned){const object=Memory.alloc(4);const chars=Memory.allocUtf8String(value);retained.push(object,chars);stringCtor(object,chars);owned.push(object);return object;}
  emit('begin',{compilerSHA256:request.compilerSHA256,requestSHA256:request.requestSHA256,fridaVersion:Frida.version,architecture:Process.arch,programType:0,unitVersion:null,timerCounterMode:'library-default',fullProgramCompiled:false});
  initLanguage();
  const kbase=Memory.alloc(0x4000);retained.push(kbase);kbaseCtor(kbase);
  const profileStrings=[];
  let count=0;
  try{
   for(const cpu of request.profiles){
    const selected=setPLC(kbase,string(cpu.device,profileStrings),string(cpu.cpu,profileStrings));
    if(selected!==1)throw new Error('SetPLC failed for '+cpu.device+' '+cpu.cpu);
    const descriptor=kbase.add(0x2d50).readPointer();
    emit('profile',{device:cpu.device,cpu:cpu.cpu,selector:descriptor.add(0x44).readU32(),familyMaskLow:descriptor.add(0x68).readU32(),familyMaskHigh:descriptor.add(0x6c).readU32()});
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
