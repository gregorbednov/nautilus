// Find literal structure displacements in executable memory. Matches are
// candidates, not proof of a receiver type; inspect each enclosing function.
// @category Omron.Research
import ghidra.app.script.GhidraScript;
import ghidra.program.model.address.Address;
import ghidra.program.model.listing.*;
import ghidra.program.model.mem.MemoryBlock;
import com.google.gson.*;
import java.nio.file.*;
import java.nio.charset.StandardCharsets;

public class ExportCJ1FieldReferences extends GhidraScript {
 public void run() throws Exception {
  String[] args=getScriptArgs();
  if(args.length<2)throw new IllegalArgumentException("OUTPUT.json HEX-DISPLACEMENT...");
  Path output=Paths.get(args[0]);if(Files.exists(output))throw new IllegalArgumentException("output exists");
  JsonObject root=new JsonObject();root.addProperty("schemaVersion",1);root.addProperty("engine","ghidra");
  root.addProperty("engineVersion",ghidra.framework.Application.getApplicationVersion());
  root.addProperty("sourceSHA256",currentProgram.getExecutableSHA256());
  root.addProperty("limitations","Literal bytes in executable blocks only; receiver type and instruction boundaries require verification.");
  JsonArray matches=new JsonArray();
  for(int n=1;n<args.length;n++){
   long value=Long.parseUnsignedLong(args[n].replaceFirst("^0[xX]",""),16);
   if(value>0xffffffffL)throw new IllegalArgumentException("32-bit displacement required");
   byte[] pattern=new byte[4];for(int i=0;i<4;i++)pattern[i]=(byte)(value>>>(i*8));
   for(MemoryBlock block:currentProgram.getMemory().getBlocks()){
    if(!block.isExecute()||!block.isInitialized())continue;
    Address cursor=block.getStart();
    while(cursor!=null&&cursor.compareTo(block.getEnd())<=0){
     monitor.checkCancelled();
     Address hit=currentProgram.getMemory().findBytes(cursor,block.getEnd(),pattern,null,true,monitor);
     if(hit==null)break;
     JsonObject row=new JsonObject();row.addProperty("displacement",args[n]);row.addProperty("matchAddress",hit.toString());
     Instruction instruction=getInstructionContaining(hit);
     if(instruction!=null){row.addProperty("instructionAddress",instruction.getAddress().toString());row.addProperty("instruction",instruction.toString());}
     Function function=getFunctionContaining(hit);
     if(function!=null){row.addProperty("functionEntry",function.getEntryPoint().toString());row.addProperty("function",function.getName());}
     FunctionIterator before=currentProgram.getFunctionManager().getFunctions(hit,false);
     Function previous=before.hasNext()?before.next():null;
     if(previous!=null){row.addProperty("precedingFunctionEntry",previous.getEntryPoint().toString());row.addProperty("precedingFunction",previous.getName());}
     matches.add(row);cursor=hit.add(1);
    }
   }
  }
  root.add("matches",matches);
  Files.writeString(output,new GsonBuilder().setPrettyPrinting().create().toJson(root)+"\n",StandardCharsets.UTF_8,StandardOpenOption.CREATE_NEW);
 }
}
