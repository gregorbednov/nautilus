# Importing an NB‑Designer HMI

From a Windows or Linux checkout, build `naut`, then give the importer the
directory that contains the project’s root `.nbp`.

```sh
naut hmi import-nb --out ./lift-hmi ./legacy-nb
naut hmi build -o ./lift-hmi/hmi/build ./lift-hmi
naut check ./lift-hmi
naut run ./lift-hmi
```

Open the address printed by `naut run`. The resulting Nautilus project defaults
to simulation with local BOOL tags. Its editable screen model is
`hmi/src/lib/nb-hmi.json`. The model records windows, ordered drawing primitives
and components, display states, navigation and tag bindings. The importer
preserves the source and rejects an existing destination.

The first release handles DBVersion 137, one HMI, and the Omron Ethernet UDP
profile used by the lift example. It imports referenced vector and bitmap
states and the startup logo. Legacy LB and LW registers are scoped to one
browser session. The file-selection screen lists files chosen by the operator
in the browser. Protected operations accept a validated Nautilus write token.
The legacy user and password tables are excluded from generated files.
Unsupported source features carry file, window and component context where
available. Generated `.hmi` and `.pkg` files serve as research references.

The lift example imports 11 windows, 197 components and 18 PLC tags.
`naut check` renders it in Edge at desktop and phone sizes with no diagnostics.
An interactive Edge test exercised Frame0 → ManualMode → Frame0, switched
the simulated Start bit in both directions, and opened and closed the file
selection window. A copy with a test navigation window rendered each of the
11 original windows and checked its aspect ratio and loaded images. A separate
copy exercised rejected and accepted write tokens, keypad entry, range and
precision validation, masked passwords, confirmation cancellation and
commit, and logout. Both the source project and a prebuilt copy were served
by `naut run`. The first source build on Windows took about two minutes in
the runtime test. To serve an existing build directly, set
`server.hmi: hmi/build` in a deployment copy of the manifest.

## Connecting a PLC

On the authorized bench, add this section to the generated manifest and set
the three FINS address fields to the controller and Nautilus host's actual route.
The example uses a loopback FINS emulator and enables reading only.

```yaml
nb-fins:
  endpoint: "127.0.0.1:9600"
  source: [0, 1, 0]
  destination: [0, 2, 0]
  write-allow: []
```

The driver uses the `alias` on each `NB_CIO_` or `NB_FINS_` tag. It accepts
CIO bit addresses and Work Area bit addresses such as `W2.00`. Add only
approved addresses to `write-allow` and start Nautilus with `NAUTILUS_TOKEN`
set. Enter that token via the HMI or send it in the `X-Nautilus-Token` header on
`POST /api/tags`. The driver requires a recent successful poll before accepting
a write, verifies the bit by reading it back, and only reads during startup and
reconnect. Device timeouts are reported without automatically retrying a write.

Bench verification on 2026‑11‑26 used an Omron CJ1 over FINS/UDP.
Nautilus read W2.00 as false, wrote and read back true, then restored false
and read it back. The emulator also checks startup, denial outside the
allowlist, stale quality after link loss and recovery without replay. When
the simulated PLC applies a write and drops its acknowledgment, the driver reports an
unconfirmed outcome and keeps the last confirmed value. The next successful
poll exposes the actual value without sending another write.

A loopback integration check ran the imported project through `naut run`.
The HTTP tag API refused writes before the first PLC poll, without a valid
token and outside the address allowlist. The emulator observed an immediate
read-back for an accepted write. During link loss, the API published the last
confirmed bit with stale quality and refused writes. Recovery cleared the
quality flag and resumed reads. The emulator's write count stayed unchanged
until the next explicit API request restored the original bit.

## Compiler investigation

NB‑Designer 1.49 and its simulator are x86 Windows PE programs. The example’s
source `.nbp`, `.whe`, `.lg`, `.vg` and `.bg` are XML. The generated
`HMI0.hmi` starts with `eViewHMI`, while the package contains several ELF32
ARM objects. These are distinct from the Nautilus runtime target.

Craft Anything evidence for the installed designer and simulator inventories
is `ev_4beee342055205bf56264986c97172f037fe48d765b57612a67330cd0fa8bdfe`
and `ev_b99f32fe36c809f808a69757523d2cbbaaed6797afe566dd45760bd9cfea04a1`.
The designer’s entry was decompiled by Ghidra 12.1.4 through Craft Anything
MCP in Deep mode
(`ev_daf5bb4c8a8d208348d020b3ff9ce9aebd353373bb33cdf8031e2f0e65ff5e4f`).
The entry-point evidence identifies the Windows startup path. A standalone
compiler ABI remains unverified.

| Address in NB‑Designer 1.49 | Observed reference | Craft Anything evidence |
| --- | --- | --- |
| `0x6d2c90` | The `NBProject` XML root name | `ev_982f5fbfea82c4316aabf3b6b1047708341387636700db6b49f04cb0172c2039` |
| `0x5fdc80` | Writes the `.hmi` header and offset table under `tar/<HMI>` using MFC `CFile` | `ev_74411c8e072728479e5285f8877d0445ba6651687ea844b97173d6f73c496cb4` |
| `0x619750` | Opens and reads the `.hmi` path | `ev_3bf426711821e5526535557ba586f9c11d8099c7fc8e435658ab9223d86a9bde` |
| `0x5fd538` | Construction of an `arm-linux-uclibc-gcc.exe -shared` command for a macro library | `ev_c9768127b67c5f42686ffac406ed94f51db60f543dbb3da0cc426eb26d4c6ada` |
| `0x6d3530` | Offline Simulation menu handler, command `0x802f` | `ev_346ab67250b57349b62aa5227ac3a6f2876318d85ef86c92e3ebf3b599e96340` |
| `0x6d3830`, `0x6d3910` | Resolves the accompanying `.pkg` path and starts the simulator with `CreateProcessW` | `ev_58feb3a6a6b03a650d8f65e9e44fe6ed0b5ec4c636e22b65686db64a41b15a29`, `ev_570c411e53c3c0ec61975b58d37c70f7e0b808525a39e7bea2882f4a8617ed72` |
| `0x5fc0a0` | Builds a `.whe` path using MFC strings | `ev_65658009a915578688c6276d5e11967a062297cd2c83689818f7acb189f380e7` |
| `0x619470` | Calls the archive constructor and size operation in `zipdll.dll` | `ev_7d028d4472bba6f0f1a472063f50d483ddf20d05eeed168c010c3e8f3af8790f` |
| `0x87c270` | Writes the package with MFC `CFile`, emits `PKGEXT` and seeks back to fill reserved tables | `ev_d8ba847fb6b44b1ea0b7e714c7eb19d14eab03a615e53308db80ab6580dd0bd3` |
| `0x6d18a0`, `0x6d1f20` | MFC frame routines that construct and remove an output `.pkg` path | `ev_cacf5e0efb8c4fd9397d78f4f400e5937ae8ea127cd4d46a04614adc1f80ae92`, `ev_1fa8ddef7d8874309c0f94af0c7218355e145bb85b20de26a75d7da5a2e7a0a5` |
| `0x879010`, `0x879120` | The frame callbacks start the MFC worker with `AfxBeginThread`. The worker calls the package writer at `0x87c270`. | `ev_f56540d8389452d6fb4bb66747a09a3314fd0b3aaaa9b12a3d65e5b0170c273e`, `ev_93faba217bbeeae52fb25a1f52ffc6e2bdb7919c51647fa20bac09d819d24b76` |

The supplied `newhmi.pkg` has the same six marker bytes at file offset
`0x10`. The supplied `HMI0.hmi` starts with the eight bytes written by the
`.hmi` serializer. Both observed frame callbacks select a mode and enter
`0x879010`. The worker calls `0x878c40` and `0x87cc90` before the package
writer. Both stages access MFC thread state
(`ev_5a4c6435bb55a1304ac6566edc9a739a562061dbb1211a65c8302e18d9233c9d`,
`ev_030d296e5eb3cba6b209ad01ad7909869a1043e457631a572a558567939348c8`).

The main executable imports XML load and save methods on
`CXmlDocumentWrapper` from `xmltool.dll`, whose exported classes wrap MSXML2.
`zipdll.dll` exports C++ archive classes such as `CZip` and `CUnzip`. These
two DLLs expose supporting functionality, and their inspected interfaces
show no standalone HMI compilation entry point. The DLL inventories are
`ev_bb838072dc65dd979f814bd48b611af9cb39140b63c40a378a418f6028ff051b` and
`ev_e812c3ccf49c557adac94658d438c0c2af059b4ce80920dd2825022c2b8329e0`.

The evidence supports implementing a native import and build pipeline from
the XML. A native NB panel build would also need the Omron serialization,
runtime package and macro toolchain. The Nautilus build uses its own Svelte
compiler and serves a native web HMI.

The Russian menu resource maps Offline Simulation (F5) to command
`0x802f`. The corresponding handler passes `1 "<project>.pkg"` to
`NBSimulator.exe`. Modes 2 and 3 map to the separate online test commands.
The offline path opens the supplied package and starts `NBGui.exe`. The
simulator constructs `HMI0.HMI.SIM~` and `RECIPE.RCP.SIM~` beside its
executable. The simulator evidence is
`ev_eae72edbdb08c27079d4866080a94337e72da6a1654e9fffd3f7fa65ad98d61a`
and `ev_9496fdfa8107a6f89b3a6089d16e63c6e738f8109475d6e53b30534627a60fca`.
In a workspace copy, the offline dialog accepted the sample package and
listed HMI0. The extracted `HMI0.HMI.SIM~` matched the sample’s
`tar/HMI0/HMI0.hmi` byte for byte. Selecting HMI0 and starting simulation
launched `NBGui.exe`.
In a separate bench copy with loopback device addresses, NB‑Designer 1.49
compiled the source with zero warnings and zero errors and generated both
`newhmi.pkg` and `tar/HMI0/HMI0.hmi`. The installed simulator showed HMI0
for the new package in Offline Simulation mode. Invoking F5 a second time
with this dialog open produced an existing-instance error. After selecting
HMI0 in the existing dialog, the installed simulator minimized and displayed
the original `Frame0`. It showed the automatic-mode title, six unlit floor
indicators, and the enable, start, stop, and manual-mode controls in the same
groups and order as the imported screen. The physical panel bezel belongs to
the Omron simulator chrome.
The GUI resolves `XMLName.dat` and the extracted `HMI0.HMI.SIM~` beside
its executable. Its dialog initializer reads the XML file and configures the
panel dimensions with `SetWindowPos`. The GUI evidence is
`ev_1e557865dfddc3661861952d7da7555d3ea43ce578b33936cc3078d342136f4c`,
`ev_65b98c118426cca404cb054d9c9fc976fde7c45a33513bc60aa50490d5b24661`
and `ev_9dd559ca4bf7a3017f05716e54fa4d815637276d5bea5186bd0b5b974cd43dc6`.

System register behavior is based on the `LB.xml` and `LW.xml` installed with
NB‑Designer 1.49. In particular, LB9000–9009 start ON, LW9180 and LW9190 show the
input limits, and the original login and logout triggers map to Nautilus
authorization. For decimal Number Input controls, both the LW value and
the configured limits are fixed-point integers. The browser formats them
using the configured number of decimal places and compares the entered
number against the raw limits before writing LW.
Password fields mask the value and the on-screen keyboard echo.
Number inputs with `LimitSwitch="Off"` use the configured literal limits.
The address saved by NB‑Designer for the disabled indirect limit is ignored.
The isolated simulator also opened the bench copy after adding the installed
`picture` directory. In this session, `Frame0 → ManualMode → Frame0`
preserved the enable and start switch states. The floor 1 request changed
its button bevel and left the separate floor indicator as it was. The
original brake button changed its caption from “Тормоз выкл.” to
“Тормоз вкл.” and showed its pressed graphic on one click.

These comparisons exposed two import defects. Graphics under `Group/GParts`
were skipped and vector primitives were drawn in the wrong stacking order.
The two captions of a switch are stored in the reverse order of its graphic
states. The importer now preserves the loaded graphics, paints their vector
layers back to front, and maps the switch captions to the matching bit states.
The reimported sample passed `naut check` with no errors or warnings. Edge
loaded the three grouped assets in ManualMode, exercised both brake and start
bit transitions against the simulated tag API, returned to Frame0 and opened
and dismissed the file chooser.

A second import of the unchanged sample produced the same 23 files byte for
byte. The dialog fixture uses a number input cloned from the sample and local
`LW10`. The installed NB‑Designer compiled the fixture. In standalone offline
simulation, its keyboard showed `Max: 50.00` and `Min: 0.00` for raw limits
`5000` and `0` with two decimal places. The first digit replaced the previous
value, the field preview followed the keyboard draft, and Enter committed
`5.1` as `5.10`. Enter with `99` left the keyboard open, and cancel restored
`5.10`. With confirmation enabled on Start, Cancel left the button released
and Yes switched it to the pressed state.

In a second compiled fixture, the same number input required a user level
and enabled `AutoPWD`. Touching it while signed out in Omron opened the
stock Login Window and left the number keyboard closed. Nautilus maps that
prompt to its write-token dialog. A successful token check allows a new
operator action. The source HMI's user table remains local to the Omron
project.

Edge verified that the Nautilus token dialog opened before the number
keyboard, an invalid token left the field locked, a valid token enabled
editing, and logout locked the field again.

The final build passed `naut check` for the unchanged example with zero
errors and warnings. Edge loaded the resources and checked the aspect ratio
of each of the 11 original windows. It reproduced the numeric draft, range
rejection and cancel behavior, both Start confirmation outcomes, and the
authorization gate. The loopback FINS/UDP integration check passed on the
same build.

Reference manuals
[NB‑Designer V106](https://files.omron.eu/downloads/latest/manual/en/v106_nb-designer_software_operation_manual_en.pdf)
and
[Omron FINS W342](https://assets.omron.eu/downloads/latest/manual/en/w342_cs_cj_cp_nsj_communications_commands_reference_manual_en.pdf?v=9).
