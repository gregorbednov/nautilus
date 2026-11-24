# Minimal source to complete N1 without FB

`n1-no-fb.cxt` is an independently authored minimal source fixture: one
unprotected task256 named `Probe`, startup1, one section/rung containing END,
explicit empty global/local symbol and instance lists, zero FB definitions.
It is not a CX-One save or the live installation's source.

Both `.n1` files are unchanged extractions from the `default` case in
`../../codec/testdata/header-memory-events.jsonl.gz`, events6 (CJ1M CPU21) and
37 (CJ1H-H CPU65). The device/case identity is authoritative. Native Program::Compile
and file writer returned1; file errorCount0. The capsule pins compiler SHA-256,
request, script, worker and emitted bytes. No new native run or PLC connection
was needed for this extension.

Each file is2624 bytes, SHA-256
`598e01ac119b890c94f5495e267269fa9546aff32e130f481d68e1ed35601b84`.
The files are identical across these two recorded CPU profiles. Tests compare
the entire compiler output, including zero auxiliary offsets/memory fields,
task configuration, directory and task wrapper. Settings use the existing
explicit example with `definitionNumbers: {}` and zero next-enable state.

The linker validates empty instance records and symbol declarations rather
than dropping unmatched FB data. The standalone FB-prefix command still
requires nonempty FB sections. This evidence qualifies offline linking for
the existing task/instruction scope, not a whole vendor CXP frontend, live
source equivalence, initialization, wire transfer or execution.
