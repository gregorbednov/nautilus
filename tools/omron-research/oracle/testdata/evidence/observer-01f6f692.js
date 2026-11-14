'use strict';

// Research-only Frida observer for the pinned CX-Pdata.dll build.
// No native compiler functions are called, arguments changed, or PLC APIs hooked.
// The launcher verifies the on-disk DLL hash before loading this script.
const moduleName = 'CX-Pdata.dll';
const maxEvents = 20000;
const maxBytes = 4 * 1024 * 1024;
let sequence = 0;
let installed = false;
let totalBytes = 0;
let droppedEvents = 0;

function emit(kind, fields) {
    if (sequence >= maxEvents && kind !== 'capture-ended') {
        droppedEvents++;
        return;
    }
    console.log('CJ1_ORACLE ' + JSON.stringify(Object.assign({
        schemaVersion: 1, sequence: sequence++, kind: kind,
        pid: Process.id, thread: Process.getCurrentThreadId()
    }, fields)));
}

function hex(address, length) {
    if (!Number.isInteger(length) || length < 0 || length > maxBytes)
        throw new Error('capture size outside bounds: ' + length);
    if (length === 0) return '';
    if (address.isNull()) throw new Error('null capture pointer');
    return bufferHex(address.readByteArray(length));
}

function bufferHex(buffer) {
    const length = buffer.byteLength;
    if (length > maxBytes || totalBytes + length > 16 * maxBytes)
        throw new Error('total capture byte budget exhausted');
    totalBytes += length;
    return Array.from(new Uint8Array(buffer),
        value => value.toString(16).padStart(2, '0')).join('');
}

function text(address) {
    if (address.isNull()) return null;
    let value = '';
    for (let index = 0; index < 256; index++) {
        const byte = address.add(index).readU8();
        if (byte === 0) return value;
        value += String.fromCharCode(byte);
    }
    throw new Error('unterminated string exceeds capture bound');
}

function guarded(kind, action) {
    try { action(); }
    catch (error) { emit('capture-error', { operation: kind, message: String(error) }); }
}

function profile(kbase) {
    const descriptor = kbase.add(0x2d50).readPointer();
    return descriptorProfile(descriptor);
}

function descriptorProfile(descriptor) {
    return {
        descriptor: descriptor.toString(),
        selector: descriptor.add(0x44).readU32(),
        familyMaskLow: descriptor.add(0x68).readU32(),
        familyMaskHigh: descriptor.add(0x6c).readU32()
    };
}

function install(module) {
    if (installed) return;
    if (Process.arch !== 'ia32' || Process.pointerSize !== 4)
        throw new Error('this observer requires the x86 CX-Programmer process');
    installed = true;
    const at = rva => module.base.add(rva);
    emit('module', { name: module.name, base: module.base.toString(),
        path: module.path, size: module.size, arch: Process.arch,
        expectedSHA256: 'a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141' });

    Interceptor.attach(at(0x1ba4b2), {
        onEnter(args) {
            this.kbase = this.context.ecx;
            guarded('FindPLCAndInitialise arguments', () => {
                this.device = text(args[0].readPointer());
                this.cpu = text(args[1].readPointer());
            });
        },
        onLeave(result) {
            if (result.toInt32() === 0) return;
            guarded('FindPLCAndInitialise result', () => emit('cpu-profile', {
                device: this.device, cpu: this.cpu, profile: profile(this.kbase)
            }));
        }
    });

    Interceptor.attach(at(0x16d735), {
        onEnter(args) {
            this.instance = this.context.ecx;
            this.kbase = args[0];
        },
        onLeave(result) {
            guarded('EncodeFunction', () => {
                const instance = this.instance;
                const instruction = instance.add(0x2c).readPointer();
                if (instruction.isNull()) return;
                const count = instance.add(0x34).readU8();
                const buffer = instance.add(0x0c).readPointer();
                emit('instruction', {
                    instance: instance.toString(), result: result.toInt32(),
                    profile: profile(this.kbase),
                    mnemonic: text(instruction.add(0x0c).readPointer()),
                    flags: instance.add(0x30).readU32(),
                    count: count,
                    // x86 process memory order, NOT asserted to be PLC wire order.
                    containerHexLE: buffer.isNull() ? null : hex(buffer, count * 4)
                });
            });
        }
    });

    Interceptor.attach(at(0x1dd3e0), {
        onEnter(args) {
            guarded('DoFunctionCheck', () => {
                const instance = args[0].add(0x10).readPointer();
                const instruction = instance.add(0x2c).readPointer();
                const buffer = instance.add(0x0c).readPointer();
                const count = instance.add(0x34).readU8();
                emit('check-instruction', {
                    program: this.context.ecx.toString(), instance: instance.toString(),
                    profile: profile(instance.add(8).readPointer()),
                    mnemonic: text(instruction.add(0x0c).readPointer()),
                    flags: instance.add(0x30).readU32(), count: count,
                    containerHexLE: buffer.isNull() ? null : hex(buffer, count * 4)
                });
            });
        }
    });

    Interceptor.attach(at(0x1d68cf), {
        onEnter(args) {
            this.program = this.context.ecx;
            this.output = args[0];
        },
        onLeave(end) {
            guarded('GetProgramBinary', () => {
                const length = end.sub(this.output).toInt32();
                emit('program', { program: this.program.toString(),
                    task: this.program.add(0x204).readU32(),
                    declaredContainerCount: this.program.add(0x1ec).readU32(),
                    byteLength: length, containerHexLE: hex(this.output, length) });
            });
        }
    });

    Interceptor.attach(at(0x16f2f9), {
        onEnter(args) {
            guarded('InstructionInst::Serialize', () => {
                const instance = this.context.ecx;
                const instruction = instance.add(0x2c).readPointer();
                if (instruction.isNull()) return;
                const buffer = instance.add(0x0c).readPointer();
                const count = instance.add(0x34).readU8();
                emit('serialized-instruction', {
                    instance: instance.toString(), profile: descriptorProfile(args[1]),
                    mnemonic: text(instruction.add(0x0c).readPointer()),
                    sourceSpelling: text(instance.add(0x38).readPointer()),
                    flags: instance.add(0x30).readU32(), count: count,
                    containerHexLE: buffer.isNull() ? null : hex(buffer, count * 4)
                });
            });
        }
    });

    Interceptor.attach(at(0x1c28ef), {
        onEnter(args) {
            guarded('CompileMultipleProgramsAndProduceBinaryFile', () => {
                this.outputPath = text(args[6]) || text(this.context.ecx.add(0x2ce8).readPointer());
                emit('compile-enter', {
                    profile: profile(this.context.ecx),
                    explicitOutputPath: text(args[6]),
                    defaultOutputPath: text(this.context.ecx.add(0x2ce8).readPointer()),
                    mode: args[2].toInt32(), task: args[4].toUInt32()
                });
            });
        },
        onLeave(result) {
            emit('compile-leave', { result: result.toInt32() });
            if (result.toInt32() !== 1 || !this.outputPath) return;
            guarded('compiler file capture', () => {
                const file = new File(this.outputPath, 'rb');
                try {
                    const bytes = file.readBytes(maxBytes + 1);
                    emit('compiler-file', { path: this.outputPath,
                        byteLength: bytes.byteLength, fileHex: bufferHex(bytes) });
                } finally { file.close(); }
            });
        }
    });
    emit('ready', { hooks: ['FindPLCAndInitialise', 'DoFunctionCheck', 'EncodeFunction', 'GetProgramBinary', 'InstructionInst::Serialize',
        'CompileMultipleProgramsAndProduceBinaryFile'] });
}

// Module observer also handles a DLL loaded after attaching to the application.
const moduleObserver = Process.attachModuleObserver({ onAdded(module) {
    if (module.name.toLowerCase() === moduleName.toLowerCase())
        guarded('install', () => install(module));
} });
emit('observer-started', { waitingFor: moduleName });
setTimeout(() => {
    moduleObserver.detach();
    Interceptor.detachAll();
    emit('capture-ended', { totalBytes: totalBytes, droppedEvents: droppedEvents });
}, 180000);
