'use strict';
// Decode supplied captured instruction containers using normal library objects.
// No compiled flags, decoded operands or internal program buffers are forged.
let sequence=0;
function emit(kind,fields){console.log('CJ1_INSTRUCTION '+JSON.stringify(Object.assign({schemaVersion:1,kind,sequence:sequence++},fields)));}
function text(address){let value='';for(let i=0;i<1024;i++){const n=address.add(i).readU8();if(!n)return value;value+=String.fromCharCode(n);}throw new Error('string bound exceeded');}
setImmediate(()=>{
 try{
  if(Process.arch!=='ia32' || request.probe!=='fb-native-decode-v1' || request.programType!==32 || request.timerCounterMode!=='bcd' || request.cases.length>512)throw new Error('unsupported decode request');
  const library=Module.load(request.binary);library.ensureInitialized();
  const native=(rva,result,args,abi='thiscall')=>new NativeFunction(library.base.add(rva),result,args,abi);
  const initLanguage=native(0x1ba0e1,'void',[],'mscdecl');
  const stringCtor=native(0x24e2de,'pointer',['pointer','pointer']);
  const stringDtor=native(0x24e2a2,'void',['pointer']);
  const kbaseCtor=native(0x1b96e5,'pointer',['pointer']);
  const kbaseDtor=native(0x1b9cf0,'void',['pointer']);
  const setPLC=native(0x1ba36d,'int',['pointer','pointer','pointer']);
  const setBCD=native(0x1cf6b3,'void',['pointer']);
  const isBCD=native(0x125e0,'int',['pointer']);
  const isBinary=native(0x12600,'int',['pointer']);
  const programCtor=native(0x1d5677,'pointer',['pointer']);
  const programDtor=native(0x1d5c77,'void',['pointer']);
  const setProgramType=native(0x1d6216,'void',['pointer','int']);
  const decompile=native(0x1e8b11,'void',['pointer','pointer','pointer','int']);
  const getText=native(0x16c2ae,'void',['pointer','pointer','int']);
  // GetRuntimeClass returns this fixed CRunTimeClass; nObjectSize is at +4.
  const objectSize=library.base.add(0x2726b8+4).readU32();
  if(objectSize<0x2f0 || objectSize>0x1000)throw new Error('unreviewed PLC_Program size');
  const retained=[];
  function string(value,owned){const object=Memory.alloc(4),chars=Memory.allocUtf8String(value);retained.push(object,chars);stringCtor(object,chars);owned.push(object);return object;}
  emit('begin',{probe:request.probe,compilerSHA256:request.compilerSHA256,requestSHA256:request.requestSHA256,fridaVersion:Frida.version,architecture:Process.arch,programType:32,unitVersion:null,timerCounterMode:'bcd',fullProgramCompiled:false,objectSize});
  initLanguage();
  const kbase=Memory.alloc(0x4000);retained.push(kbase);kbaseCtor(kbase);
  const owned=[];
  let count=0;
  try{
   for(const cpu of request.profiles){
    if(setPLC(kbase,string(cpu.device,owned),string(cpu.cpu,owned))!==1)throw new Error('SetPLC failed');
    setBCD(kbase);
    if(isBCD(kbase)!==1 || isBinary(kbase)!==0)throw new Error('timer mode readback mismatch');
    const descriptor=kbase.add(0x2d50).readPointer();
    emit('profile',{device:cpu.device,cpu:cpu.cpu,selector:descriptor.add(0x44).readU32(),familyMaskLow:descriptor.add(0x68).readU32(),familyMaskHigh:descriptor.add(0x6c).readU32(),timerModeBcd:1,timerModeBinary:0});
    for(const test of request.cases){
     if(!Array.isArray(test.code) || !test.code.length || test.code.length>128 || test.code.some(x=>!Number.isInteger(x)||x<0||x>0xffffffff))throw new Error('invalid code input');
     const program=Memory.alloc(objectSize),buffer=Memory.alloc(test.code.length*4);retained.push(program,buffer);
     test.code.forEach((word,i)=>buffer.add(i*4).writeU32(word));
     programCtor(program);
     try{
      setProgramType(program,32);
      decompile(program,kbase,buffer,test.code.length);
      // AddToProgram121d62a8 adds to CPtrList at +210. MFC node is next/prev/data.
      const list=program.add(0x210),expected=list.add(12).readU32();
      if(expected>128)throw new Error('decoded list bound exceeded');
      let node=list.add(4).readPointer(),offset=0;
      const seen=new Set(),decoded=[],words=[];
      while(!node.isNull()){
       if(decoded.length>=expected || seen.has(node.toString()))throw new Error('invalid decoded list');seen.add(node.toString());
       const instruction=node.add(8).readPointer(),length=instruction.add(0x34).readU8();
       if(!length || length>128 || offset+length>test.code.length)throw new Error('invalid decoded span');
       const nativeBuffer=instruction.add(0xc).readPointer();if(nativeBuffer.isNull())throw new Error('missing decoded code');
       const span=[];for(let i=0;i<length;i++)span.push(nativeBuffer.add(i*4).readU32());
       const outputOwned=[],output=string('',outputOwned);
       let line;try{getText(instruction,output,0);line=text(output.readPointer());}finally{stringDtor(output);}
       decoded.push({text:line,offsetWords:offset,words:span,known:!instruction.add(0x2c).readPointer().isNull()});
       words.push(...span);offset+=length;node=node.readPointer();
      }
      if(decoded.length!==expected || offset!==test.code.length || words.some((word,i)=>word!==test.code[i]))throw new Error('incomplete or changed decoded body');
      emit('case',{id:test.id,device:cpu.device,cpu:cpu.cpu,mnemonic:test.mnemonic,operands:test.operands,sourceSHA256:test.sourceSHA256,sourceOffsetWords:test.sourceOffsetWords,accepted:decoded.every(x=>x.known),words,decoded});count++;
     }finally{programDtor(program);}
    }
   }
  }finally{for(const object of owned.reverse())stringDtor(object);kbaseDtor(kbase);}
  emit('done',{caseCount:count});
 }catch(error){emit('error',{message:String(error),stack:error.stack});}
});
