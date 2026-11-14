#!/usr/bin/env python3
"""Reproducible, static CX-One inventory. Never executes vendor binaries.

inventory ARCHIVE: hash source ZIP and inventory relevant CABs/CPU profiles.
compiler DLL: recover instruction-constructor facts from the pinned CX v4.40
cx_pdata.dll build, using objdump. These are research records, not a statement
of CPU applicability, native opcode layout, or implemented instruction support.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import struct
import subprocess
import zipfile


COMPILER_SHA256 = "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141"
CONSTRUCTOR = 0x12012EC0
STRING_CONSTRUCTOR = 0x1224E2DE


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


class PE:
    """Minimal bounded PE32 address/exports reader; no loading or execution."""

    def __init__(self, path):
        self.data = path.read_bytes()
        if self.data[:2] != b"MZ":
            raise ValueError("not an MZ executable")
        nt = self.unpack("<I", 0x3C)[0]
        if self.data[nt:nt + 4] != b"PE\0\0":
            raise ValueError("not a PE executable")
        machine, count = self.unpack("<HH", nt + 4)
        optional_size = self.unpack("<H", nt + 20)[0]
        optional = nt + 24
        if machine != 0x14C or self.unpack("<H", optional)[0] != 0x10B:
            raise ValueError("expected x86 PE32")
        self.base = self.unpack("<I", optional + 28)[0]
        self.sections = [self.unpack("<IIII", optional + optional_size + i * 40 + 8)
                         for i in range(count)]
        self.export_rva = self.unpack("<I", optional + 96)[0]

    def unpack(self, fmt, offset):
        size = struct.calcsize(fmt)
        if offset < 0 or offset + size > len(self.data):
            raise ValueError("truncated PE structure")
        return struct.unpack_from(fmt, self.data, offset)

    def offset(self, address):
        rva = address - self.base
        for virtual_size, virtual_address, size, pointer in self.sections:
            if virtual_address <= rva < virtual_address + max(size, virtual_size):
                result = pointer + rva - virtual_address
                if result >= pointer + size or result >= len(self.data):
                    raise ValueError("address has no file-backed bytes")
                return result
        raise ValueError(f"unmapped PE address: {address:#x}")

    def string(self, address):
        offset = self.offset(address)
        end = self.data.find(b"\0", offset, offset + 512)
        if end < 0:
            raise ValueError("unterminated PE string")
        return self.data[offset:end].decode("ascii")

    def exports(self):
        directory = self.offset(self.base + self.export_rva)
        count = self.unpack("<I", directory + 24)[0]
        funcs, names, ordinals = self.unpack("<III", directory + 28)
        result = {}
        for index in range(count):
            name = self.unpack("<I", self.offset(self.base + names) + index * 4)[0]
            ordinal = self.unpack("<H", self.offset(self.base + ordinals) + index * 2)[0]
            function = self.unpack("<I", self.offset(self.base + funcs) + ordinal * 4)[0]
            result[self.string(self.base + name)] = self.base + function
        return result


def compiler(path, objdump):
    sha = digest(path)
    if sha != COMPILER_SHA256:
        raise ValueError("compiler differs from researched build; recover and review its layout first")
    pe = PE(path)
    exports = pe.exports()
    constructors = [addr for name, addr in exports.items()
                    if name.startswith("??0CInstruction@@QAE@HH")]
    if constructors != [CONSTRUCTOR]:
        raise ValueError("instruction constructor no longer matches researched layout")
    # The full disassembly is transient. Nothing from vendor machine code is
    # copied into the source tree; only facts below are emitted.
    text = subprocess.check_output([objdump, "-d", str(path)], text=True)
    stack, strings, records = [], [], []
    constructor_calls = 0
    for line in text.splitlines():
        if "\tpush" in line:
            match = re.search(r"\$0x([0-9a-f]+)", line)
            stack.append(int(match[1], 16) if match else None)
        if "\tcall" not in line:
            continue
        if f"0x{STRING_CONSTRUCTOR:x}" in line and stack and stack[-1] is not None:
            try:
                strings.append(pe.string(stack[-1]))
            except (ValueError, UnicodeDecodeError):
                pass
        if f"0x{CONSTRUCTOR:x}" in line:
            constructor_calls += 1
            args = stack[::-1]
            if len(strings) >= 2 and len(args) >= 42:
                # Constructor stores stack +4C/+50 in CInstruction +C0/+C1;
                # GetNumberOfOperands reads +C0. Unknown dynamic args are null.
                records.append({
                    "callVA": line.split(":")[0].strip(),
                    "names": strings[-2:],
                    "operands": args[17], "minimumOperands": args[18],
                    "rawFunctionIdentifiers": args[24:27],
                    "rawFamilyMask": args[22:24],
                    "rawHandlerVAs": [f"{a:08x}" if a is not None else None
                                      for a in args[36:42]],
                })
            strings = []
        # Only the immediate argument suffix following the last helper call
        # is decoded. No calling convention or dynamic helper result is guessed.
        stack = []
    if constructor_calls != 1505 or len(records) != 1504:
        raise ValueError(f"incomplete extraction: {constructor_calls} calls, {len(records)} records")
    return {
        "source": path.name, "sha256": sha,
        "status": "statically recovered; CPU applicability and executable encoding unverified",
        "constructorVA": f"{CONSTRUCTOR:08x}",
        "constructorCalls": constructor_calls, "records": records,
        "exports": {name: f"{addr:08x}" for name, addr in exports.items()
                    if any(term in name for term in ("Compile", "Decompile", "Binary", "Assemble"))},
    }


def inventory(path):
    with zipfile.ZipFile(path) as archive:
        files = archive.infolist()
        relevant = [entry for entry in files
                    if (len(entry.filename.split("/")) == 2
                        and entry.filename.lower().endswith(".cab")
                        and any(s in entry.filename.lower() for s in ("cxp", "cxs", "cxserver")))
                    or ("/cps (fw)/" in entry.filename.lower()
                        and any(s in entry.filename.lower() for s in ("cj1m", "cj1h")))]
        records = [{"path": e.filename, "bytes": e.file_size,
                    "sha256": hashlib.sha256(archive.read(e)).hexdigest()}
                   for e in relevant]
    return {"source": path.name, "sha256": digest(path),
            "zipEntries": len(files), "relevant": records}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("inventory", "compiler"))
    parser.add_argument("source", type=Path)
    parser.add_argument("--objdump", default="objdump")
    parser.add_argument("--out", type=Path, help="new output file; stdout if omitted")
    args = parser.parse_args()
    result = inventory(args.source) if args.mode == "inventory" else compiler(args.source, args.objdump)
    text = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.out:
        with args.out.open("x", encoding="utf-8") as output:
            output.write(text)
    else:
        print(text, end="")


if __name__ == "__main__":
    main()
