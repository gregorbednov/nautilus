// Export selected functions from a local research binary; never execute it.
// @category Omron.Research
import ghidra.app.script.GhidraScript;
import ghidra.app.decompiler.*;
import ghidra.program.model.address.Address;
import ghidra.program.model.listing.*;
import ghidra.program.model.symbol.*;
import com.google.gson.*;
import java.nio.file.*;
import java.nio.charset.StandardCharsets;

public class ExportCJ1Functions extends GhidraScript {
    public void run() throws Exception {
        String[] args = getScriptArgs();
        if (args.length < 2) throw new IllegalArgumentException("OUTPUT.json ADDRESS...");
        Path output = Paths.get(args[0]);
        if (Files.exists(output)) throw new IllegalArgumentException("output exists: " + output);
        JsonObject root = new JsonObject();
        root.addProperty("schemaVersion", 1);
        root.addProperty("engine", "ghidra");
        root.addProperty("engineVersion", ghidra.framework.Application.getApplicationVersion());
        root.addProperty("sourceSHA256", currentProgram.getExecutableSHA256());
        root.addProperty("language", currentProgram.getLanguageID().toString());
        root.addProperty("imageBase", currentProgram.getImageBase().toString());
        root.addProperty("status", "research-only; CPU encoding unverified");
        root.addProperty("analysisMode", "selected-functions; no whole-program auto-analysis");
        root.addProperty("limitations", "Types and calling conventions may be incomplete; compare assembly before assigning semantics.");
        JsonArray functions = new JsonArray();
        DecompInterface decompiler = new DecompInterface();
        try {
            if (!decompiler.openProgram(currentProgram)) throw new IllegalStateException(decompiler.getLastMessage());
            for (int i = 1; i < args.length; i++) {
                monitor.checkCancelled();
                Address address = toAddr(args[i].replaceFirst("^0[xX]", ""));
                JsonObject item = new JsonObject();
                item.addProperty("requestedAddress", args[i]);
                Function fn = getFunctionAt(address);
                if (fn == null) {
                    disassemble(address);
                    fn = createFunction(address, null);
                }
                if (fn == null) {
                    item.addProperty("error", "no function at requested entry");
                    functions.add(item);
                    continue;
                }
                item.addProperty("entry", fn.getEntryPoint().toString());
                item.addProperty("name", fn.getName());
                item.addProperty("signature", fn.getSignature().toString());
                JsonArray calls = new JsonArray();
                JsonArray instructions = new JsonArray();
                InstructionIterator it = currentProgram.getListing().getInstructions(fn.getBody(), true);
                while (it.hasNext()) {
                    Instruction insn = it.next();
                    JsonObject line = new JsonObject();
                    line.addProperty("address", insn.getAddress().toString());
                    line.addProperty("text", insn.toString());
                    instructions.add(line);
                    for (Reference ref : insn.getReferencesFrom()) {
                        if (!ref.getReferenceType().isCall()) continue;
                        JsonObject call = new JsonObject();
                        call.addProperty("site", insn.getAddress().toString());
                        call.addProperty("target", ref.getToAddress().toString());
                        Function callee = getFunctionAt(ref.getToAddress());
                        if (callee != null) call.addProperty("name", callee.getName());
                        calls.add(call);
                    }
                }
                item.add("instructions", instructions);
                item.add("calls", calls);
                DecompileResults result = decompiler.decompileFunction(fn, 30, monitor);
                item.addProperty("decompileCompleted", result.decompileCompleted());
                if (result.decompileCompleted()) item.addProperty("pseudocode", result.getDecompiledFunction().getC());
                else item.addProperty("error", result.getErrorMessage());
                functions.add(item);
                println("Exported " + fn.getName() + " at " + address);
            }
        } finally { decompiler.dispose(); }
        root.add("functions", functions);
        Files.writeString(output, new GsonBuilder().setPrettyPrinting().create().toJson(root) + "\n", StandardCharsets.UTF_8, StandardOpenOption.CREATE_NEW);
    }
}
