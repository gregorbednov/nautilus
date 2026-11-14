// Recover additional constructor facts from a reviewed call-site inventory.
// @category Omron.Research
import ghidra.app.script.GhidraScript;
import ghidra.app.util.PseudoDisassembler;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.address.Address;
import ghidra.program.model.scalar.Scalar;
import com.google.gson.*;
import java.nio.file.*;
import java.nio.charset.StandardCharsets;
import java.util.*;

public class ExportCJ1Catalog extends GhidraScript {
 public void run() throws Exception {
  String[] args=getScriptArgs();
  if(args.length!=2) throw new IllegalArgumentException("FACTS.json OUTPUT.json");
  JsonObject facts=JsonParser.parseString(Files.readString(Paths.get(args[0]),StandardCharsets.UTF_8).replaceFirst("^\\ufeff", "")).getAsJsonObject();
  if(!currentProgram.getExecutableSHA256().equalsIgnoreCase(facts.get("sha256").getAsString())) throw new IllegalArgumentException("source hash mismatch");
  Path output=Paths.get(args[1]);
  if(Files.exists(output)) throw new IllegalArgumentException("output exists");
  JsonObject root=new JsonObject();
  root.addProperty("schemaVersion",1);
  root.addProperty("engine","ghidra");
  root.addProperty("engineVersion",ghidra.framework.Application.getApplicationVersion());
  root.addProperty("sourceSHA256",currentProgram.getExecutableSHA256());
  root.addProperty("status","static facts only; CPU applicability and executable encoding unverified");
  JsonArray records=new JsonArray();
  PseudoDisassembler decoder=new PseudoDisassembler(currentProgram);
  int failed=0;
  for(JsonElement element:facts.getAsJsonArray("records")) {
   monitor.checkCancelled();
   JsonObject known=element.getAsJsonObject();
   Address call=toAddr(known.get("callVA").getAsString());
   JsonObject record=new JsonObject();
   record.add("callVA",known.get("callVA"));
   record.add("names",known.get("names"));
   try {
    Instruction finalCall=decoder.disassemble(call);
    if(!finalCall.getFlowType().isCall() || finalCall.getFlows().length!=1 || !finalCall.getFlows()[0].equals(toAddr("12012ec0"))) throw new IllegalStateException("constructor call mismatch");
    ArrayList<JsonElement> pushes=new ArrayList<>();
    JsonElement eax=JsonNull.INSTANCE;
    Address cursor=call.subtract(1024);
    while(cursor.compareTo(call)<0) {
     Instruction insn;
     try { insn=decoder.disassemble(cursor); } catch(Exception e) { cursor=cursor.add(1); pushes.clear(); eax=JsonNull.INSTANCE; continue; }
     if(insn==null) { cursor=cursor.add(1); pushes.clear(); eax=JsonNull.INSTANCE; continue; }
     String mnemonic=insn.getMnemonicString().toUpperCase(Locale.ROOT);
     if(insn.getFlowType().isCall()) {
      Address[] targets=insn.getFlows();
      String target=targets.length==1 ? targets[0].toString() : "";
      int arity=target.equals("1203a708") ? 7 : target.equals("1203a7a3") ? 8 : 0;
      if(arity!=0) {
       if(pushes.size()<arity) { eax=JsonNull.INSTANCE; } else {
       JsonObject table=new JsonObject();
       table.addProperty("builder",target);

       JsonArray values=new JsonArray();
       for(int i=0;i<arity;i++) values.add(pushes.get(pushes.size()-1-i));
              table.add("rawArguments",values);
       int bits=target.equals("1203a708") || values.get(0).getAsInt()==3 ? 32 : 16;
       int countIndex=target.equals("1203a708") ? 0 : 1;
       int count=values.get(countIndex).getAsInt();
       if(count<0 || count>6) throw new IllegalStateException("invalid table length");
       table.addProperty("elementBits",bits);
       JsonArray words=new JsonArray();
       for(int w=0;w<count;w++) words.add(values.get(countIndex+1+w).getAsLong() & (bits==32 ? 0xffffffffL : 0xffffL));
       table.add("words",words);
       eax=table; }
      } else { pushes.clear(); eax=JsonNull.INSTANCE; }
     }
     if(mnemonic.equals("ADD") && insn.getRegister(0)!=null && insn.getRegister(0).getName().equalsIgnoreCase("ESP")) {
      Scalar amount=insn.getScalar(1);
      if(amount==null || amount.getUnsignedValue()%4!=0) throw new IllegalStateException("unrecognised stack cleanup");
      int count=(int)amount.getUnsignedValue()/4;
      if(count>pushes.size()) pushes.clear();
      else for(int i=0;i<count;i++) pushes.remove(pushes.size()-1);
     }
     if(!insn.getFlowType().isCall()) {
      for(Object object:insn.getResultObjects()) {
       if(object instanceof ghidra.program.model.lang.Register) {
        String name=((ghidra.program.model.lang.Register)object).getName();
        if(name.equalsIgnoreCase("EAX") || name.equalsIgnoreCase("AX") || name.equalsIgnoreCase("AL") || name.equalsIgnoreCase("AH")) eax=JsonNull.INSTANCE;
       }
      }
     }
     if(mnemonic.equals("PUSH")) {
      Scalar scalar=insn.getScalar(0);
      if(scalar!=null) pushes.add(new JsonPrimitive(scalar.bitLength()==8 ? scalar.getSignedValue() & 0xffffffffL : scalar.getUnsignedValue()));
      else if(insn.getRegister(0)!=null && insn.getRegister(0).getName().equalsIgnoreCase("EAX")) pushes.add(eax);
      else pushes.add(JsonNull.INSTANCE);
     }
     cursor=cursor.add(insn.getLength());
    }
    if(!cursor.equals(call)) throw new IllegalStateException("instruction alignment mismatch");
    Collections.reverse(pushes);
    if(pushes.size()<64) throw new IllegalStateException("only "+pushes.size()+" constructor arguments recovered");
    for(int i=0;i<6;i++) {
     JsonElement expected=known.getAsJsonArray("rawHandlerVAs").get(i);
     JsonElement value=pushes.get(36+i);
     if(expected.isJsonNull() ? !value.isJsonNull() : !value.isJsonPrimitive() || value.getAsLong()!=Long.parseUnsignedLong(expected.getAsString(),16)) throw new IllegalStateException("handler argument mismatch at "+i);
    }
    if(pushes.get(17).getAsLong()!=known.get("operands").getAsLong()) throw new IllegalStateException("operand count mismatch");
    for(int i=0;i<2;i++) if(pushes.get(22+i).getAsLong()!=known.getAsJsonArray("rawFamilyMask").get(i).getAsLong()) throw new IllegalStateException("family mask mismatch");
    JsonArray arguments=new JsonArray();
    for(int index=0;index<64;index++) arguments.add(pushes.get(index));
    record.add("rawConstructorPrefix",arguments);
    JsonArray templates=new JsonArray();
    for(int index=51;index<=53;index++) {
     JsonElement value=pushes.get(index);
     if(value.isJsonPrimitive() && value.getAsLong()==0) continue;
     if(!value.isJsonObject()) throw new IllegalStateException("unresolved template argument "+index);
     JsonObject template=value.getAsJsonObject().deepCopy();
     template.addProperty("argumentIndex",index);
     template.addProperty("objectOffset",String.format("%x",0x10c+(index-51)*4));
     template.addProperty("declaredBaseLength",pushes.get(index-51+9).getAsInt());
     template.addProperty("matchesDeclaredBaseLength",template.getAsJsonArray("words").size()==pushes.get(index-51+9).getAsInt());
     templates.add(template);
    }
    record.add("templates",templates);
    record.addProperty("verifiedAgainstPriorFacts",true);
   } catch(Exception e) { record.addProperty("error",e.toString()); failed++; }
   records.add(record);
  }
  root.add("records",records);
  root.addProperty("failedRecords",failed);
  Files.writeString(output,new GsonBuilder().setPrettyPrinting().create().toJson(root)+"\n",StandardCharsets.UTF_8,StandardOpenOption.CREATE_NEW);
  println("Catalog: "+records.size()+" records; "+failed+" errors");
 }
}
