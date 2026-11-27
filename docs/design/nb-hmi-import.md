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
`NBSimulator.exe`. Mode 2 selects Indirect Online Test and mode 3 selects
Direct Online Test. The direct test starts `NBComserver.exe` and `NBGui.exe`.
An isolated simulator directory must include `NBComserver.exe` to run this
mode. The launcher extracts the Ethernet PLC driver alongside its executable
as `drv_p3.dll` for the reference configuration.
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

## Experimental NB10W panel package build

### Build and deploy a first project over Ethernet

1. Build the Nautilus CLI from the repository. On Windows, run
   `go build -o naut.exe ./cmd/naut` in PowerShell at the repository root.
2. Keep a copy of the panel's current user project, such as `newhmi.pkg`.
   Select a compatible NB10W-TW01B PKGEXT 3 file as the build reference.
   The reference supplies the runtime, stock controls, existing screens,
   and panel and PLC settings. Use a reference configured for the
   intended panel and PLC when building a connected HMI.
3. Create `panel-hmi/hmi/src/lib/nb-hmi.json` with the example below. For
   an existing project, `naut hmi import-nb --out panel-hmi <source-directory>`
   creates this model from the directory containing the `.nbp` and its
   assets. Adapt the imported windows and components to the panel compiler
   subset below. For example, the lift's browser import includes a switch
   with different captions by state, which the panel compiler rejects.

```json
{
  "version": 1,
  "name": "Nautilus on NB10W",
  "width": 800,
  "height": 480,
  "start": 0,
  "windows": [
    {
      "id": 0,
      "name": "Frame0",
      "bounds": {"x": 0, "y": 0, "w": 800, "h": 480},
      "background": "#000000",
      "parts": [
        {
          "kind": "draw",
          "name": "Title",
          "drawing": {
            "kind": "text",
            "bounds": {"x": 40, "y": 40, "w": 680, "h": 56},
            "text": {"text": "Nautilus on NB10W", "size": 32, "color": "#ffffff", "align": 0}
          }
        }
      ]
    }
  ]
}
```

4. Verify the reference, build to a new `.pkg` file, and inspect the result.
   `--offline` selects the experimental package builder tested on the
   physical NB10W-TW01B below.

   ```powershell
   .\naut.exe hmi nb-inspect .\newhmi.pkg
   .\naut.exe hmi nb-build --offline --target NB10W-TW01B --reference .\newhmi.pkg --out .\panel-test.pkg .\panel-hmi
   .\naut.exe hmi nb-inspect .\panel-test.pkg
   ```

5. Open Omron's `NBManager.exe` directly. Set the connection type to
   **Network** and enter the panel's management address and port. Use
   **Download User Data** to open `panel-test.pkg`. In NBDownload, select
   **HMI0** and **Data File**, clear the data-clearing options, and start
   the download. Wait for the success message. This transfer replaces the
   running user project. [Omron NBManager project transfer](https://automation-knowledge-base.omron.eu/support/solutions/articles/103000352732-download-and-upload-password-protected-projects-to-nb-hmi)
6. Check that the start screen and the expected PLC indications appear.
   For a rollback, transfer the saved `newhmi.pkg` the same way. If the
   panel provides the **USB → HMI** system-menu operation, Omron documents
   loading a `.pkg` from the USB host port. The trial below used Ethernet.
   [Omron USB project transfer](https://automation-knowledge-base.omron.eu/support/solutions/articles/103000352752-how-to-use-usb-memory-sticks-on-nb-hmi)

### Supported project model

```sh
naut hmi nb-inspect seed.pkg
naut hmi nb-build --offline --target NB10W-TW01B --reference seed.pkg --out test.pkg project
```

The target is the original 800×480 NB10W-TW01B, model ID 122. The input
is the version 1 Nautilus model at `project/hmi/src/lib/nb-hmi.json`.
Use a local PKGEXT 3 reference package compatible with the target panel.
Start at screen 0 and use full-screen black windows. For additional
windows, use IDs 10 through 32767. New IDs can extend the reference
directory. Gaps beyond the reference are represented by empty slots.
The compiler uses the existing screen slots when the records fit.
It can place generated bitmap and state graphics in a separate resource
area when a screen needs more room. When the model adds a screen ID or
the component records exceed the slot, the compiler appends a screen
directory with records sized for the supplied windows. Retained windows
can still reference data at its original offsets.
Growth requires the supported NB10W PKGEXT 3 resource directories.

The compiler emits plain left-aligned text drawings and `Function Key`
objects with the `Change Frame` function. Every destination must be part
of the model. A navigation button uses the stock Change Frame graphic
from a compatible button in the reference. Set its caption using `labels`
and a button-relative `labelBox`. Coordinates and bounds are in pixels.

Vector drawings support `rect`, `roundrect`, `line`, and `polygon`.
For a line, use the origin in `bounds` and the endpoint in `x2` and `y2`.
Polygon vertices in `points` are absolute screen coordinates. Closed
shapes accept a solid `fill` and a `stroke` with `strokeWidth`, using
`#RRGGBB` colors or `none`. A rounded rectangle uses `radius` in pixels.
For a vertical gradient on `rect` or `roundrect`, set `fill` to the top
color and `endFill` to the bottom color. The optional stroke has its own
color and width. The gradient is stored within the native rectangle
record. A one-pixel-high rectangle uses the starting color.
The compiler emits RGB565. It represents opaque black with the same
near-black value as NB-Designer because RGB565 zero is transparent in
the Omron vector renderer.

For a bitmap drawing, set `drawing.kind` to `image`. Set `drawing.image`
to a URL such as `/graphics/status.webp` for a file under
`project/hmi/static`, or to a base64 data URL with MIME type `image/png`,
`image/jpeg`, `image/webp` or `image/bmp`.

The compiler accepts static PNG, JPEG and WebP images, 24-bit and 32-bit
uncompressed BMP, and 16-bit RGB565 BMP with explicit color masks. It
packs the pixels into an LZW-compressed RGB565 resource. PNG and WebP
retain per-pixel alpha.
A 32-bit BMP with a populated alpha channel retains its opacity values.
A zero high byte throughout a 32-bit BI_RGB file is treated as a reserved
byte. NBSimulator scales the bitmap to its drawing bounds.

For a decimal `Number Input`, set `meta.numberFormat` to `"1"` and
`parameters` to `[minimum, maximum, decimals]`. The bounds are literal
fixed-point values from 0 through 9999, with zero to three decimal places.
For example, `[0,750,1]` represents 0.0 through 75.0. Read and write must
select the same pair of consecutive user LW registers below LW9000.
The reference supplies the numeric field graphic and the stock keyboard on
screen 3. The field uses the stock font.

A `Bit Lamp` reads one local LB or Omron CIO bit. A `Switch` in mode 2
toggles one bit and uses identical read and write bindings. Use local
bits below LB9000 or CIO addresses from 0.00 through 6143.15.

CIO bindings use the same JSON fields written by the importer. For
example, CIO120.03 uses
`{"key":"NB_CIO_0120_03","area":"CIO","index":1923,"count":1}`.
The reference package supplies the slot and node of its single
supported Omron FINS/UDP PLC. That device configuration is used for
each CIO binding in the generated package.

Switch captions use the same settings as a Change Frame button. Lamps use
unlabeled, two-state artwork. To supply custom graphics for either
component, set `assets` with both state keys, for example
`{"0":"/graphics/off.png","1":"/graphics/on.webp"}`. These references
accept the same file formats and data URLs as bitmap drawings. State 0
represents an unset bit. State 1 represents a set bit. Components without
`assets` use a compatible stock graphic from the reference.

The compiler also reads imported state SVGs under `hmi/static/nb-assets`.
It supports empty states, rectangles, rounded rectangles, lines, and
polygons in the importer's coordinate system. Rectangle gradients use
the importer's two vertical color stops. The compiler stores these
states as Omron vector records. When an SVG contains a single embedded
bitmap covering its viewBox, the compiler uses the original bitmap
bytes as input for the raster encoder. Each component uses either two
vector states or two raster states.

Number inputs and toggle switches can use the stock confirmation dialog.
Set `confirm: true` and optionally `confirmWaitSeconds` in the model.
The importer retains the project's explicit `WaitTime`. A model that
omits the timeout uses 30 seconds. Screen 7 in the reference must
contain both stock response actions writing to system word LW9370.

The compiler keeps the reference runtime, fonts, other screens and device
configuration in the resulting package. Open `test.pkg` in NBSimulator
in offline mode, select HMI0 and start the simulation.

An offline trial of the package built by `naut hmi nb-build` displayed
both Nautilus screens and their button captions. The start button opened
screen 10, and its button returned to screen 0.
Another trial wrote 54.0 to LW37. The other fields bound to it showed
54.0 on both screens. The stock keyboard rejected 99 against a 75.0 limit,
and cancel restored the committed 54.0. An LB57 switch changed the graphic
of its indicator on both screens.
With confirmation enabled in a Nautilus package, Yes committed 5.0 to
LW37 and the other field displayed 5.0. Cancel preserved that value.
For LB57, Yes lit the lamp and Cancel preserved the lit state.

`nb-inspect` checks the package offsets, the HMI record ranges and both
integrity fields. The eViewHMI resource uses a 32-bit byte sum from offset
48. Its PKGEXT 3 header encodes the MD5 of the first 16 KiB of the
resource in a 64-byte table. These checks were reproduced from three
NB-Designer packages. The format evidence includes
`ev_b7be7ea240860280acca83fa7bfc8d768c0a4e319e31454559d1fe22207a3193`
for the screen table and
`ev_9ed9f02d5e35e65459410d9c9f7fe6dc0fb407e6f7a2eca1d7460f894a06cb99`
for the digest table.

An offline trial confirmed that a CIO switch and lamps on two screens
share the value of CIO120.03. The adjacent CIO120.04 lamp stayed off.
Cancel preserved the selected bit's state.

For the FINS/UDP trial, the Nautilus package passed `nb-inspect` and opened
in Omron NBSimulator 1.49 in Direct Online Test. The window displayed
`[16]Server Not Ready` for the CIO controls. The local emulator received no
FINS requests. A control package compiled by NB-Designer, with only loopback
endpoint settings applied, also produced no FINS requests on either the test
port or the standard port 9600. An independent FINS client verified the
emulator on both ports by writing CIO120.03, reading it back, and restoring
its starting value. CIO120.04 held its original value throughout. The Nautilus
server's `internal/nbfins` loopback tests passed independently.

In a repeat run, the four device records in the simulator's shared memory
included an exact copy of the compiled FINS/UDP record. `NBGui.exe` connected
to `NBComserver.exe` on local TCP port 2008 and the server loaded `drv_p3.dll`.
The emulator received no FINS requests. The loaded Windows DLL uses
`CreateFileA`, `SetupComm` and `SetCommState` in its initialization path
(`ev_634a8743b32e1d76faa06bb09d350a5630d32dc20e06f4e96dd5d04f167c502b`,
`ev_d17988ffd21d6c825f4e408fde198346fa9070bbf8f4afe10b1763069c873177`).
Its read and write entry points use the serial file handle.

The package resource directory points to a separate ARM ELF driver for the
panel. Ghidra shows `SetupUDPComm` opening an IPv4 datagram socket and the
driver calling `sendto` and `recvfrom`
(`ev_a7cca452bc72ff9c3ac180712e679a412bb8bddaa0e78e6411be836a140d3aaa`,
`ev_68ac353cab6a4f84871c656e15c78f0f8b4857d41eaa2942eecc917e1785c90c`).
The Nautilus build kept both the ARM ELF and Windows PE driver payloads
byte for byte and moved their resource pointers by the increase in HMI size.
The package growth test now checks this behavior. FINS/UDP on the panel
requires a test of the ARM runtime. Project-wide configuration generation
and direct deployment remain open.

The control package and Nautilus prototype both dropped operations
when the dialog timed out during automated UI verification. The final
Nautilus switch trial used 300 seconds to accommodate delays between
automated actions. Both Yes and Cancel were observed before that timeout.
Omron documents that Ethernet supports transfers
of user projects, and that USB export can recover a user package on
supported panels. A physical deployment test needs the exact device
model and firmware, a compatible backup and a verified restore procedure.
[Omron Ethernet transfer guidance](https://automation-knowledge-base.omron.eu/support/solutions/articles/103000352746-how-to-connect-nb-hmi-to-peripheral-devices)
[Omron panel setup manual](https://files.omron.eu/downloads/latest/manual/en/v107_nb-series_setup_manual_en.pdf?v=1)

### Physical panel preflight

NBManager's read-only **Get Version** succeeded over Ethernet. It reported
hardware version `110`, Kernel `1764`, and Rootfs `2022`. NBManager's
**Upload User Data** can save the installed project as `.pkg` with the logo
and recipe selected. The operator identified the supplied `newhmi.pkg` as
the exact project already on the panel. A separate copy passed `nb-inspect`
for NB10W-TW01B, and its SHA-256 matched the supplied file. Use that copy for
the project restore. Omron documents restoring the user project via the
panel's USB host port. [Omron version and communication settings](https://automation-knowledge-base.omron.eu/support/solutions/articles/103000352748-how-to-update-the-nb-kernel-and-root-file-system)
[Omron project backup and restore via USB](https://automation-knowledge-base.omron.eu/support/solutions/articles/103000352752-how-to-use-usb-memory-sticks-on-nb-hmi)

A separate NB10W-TW01B test package uses an isolated Ethernet subnet, no
gateway and a single FINS/UDP endpoint for the emulator. The package passed
`nb-inspect`, and the endpoint fields and both integrity checks were verified
after compilation. The starting screen displays CIO120.03 and CIO120.04.
The second screen reads CIO120.03 again. The emulator accepts requests from
the bench HMI address and writes only to CIO120.03. The loopback self-test
verified bit and grouped reads, the write and restore sequence, and rejection
of writes to neighboring addresses.

For the university training network, the operator confirmed that the PLCs
were disconnected from the actuators. A second test package kept the panel's
network settings, used the PC as the sole FINS/UDP endpoint, and was verified
to route all five CIO bindings to the emulator. The test and restore
packages can use the same management endpoint on that network.

NBDownload successfully transferred the Nautilus package to the physical
NB10W-TW01B as user project data. On startup the ARM runtime sent FINS/UDP
`0104` requests to read CIO word 120. The emulator was extended to return
the requested memory area code and data for each item. Subsequent requests
received end code `0000` and the initial word value `0010` (hexadecimal).
[Omron FINS communications manual, section 5-3-5](https://files.omron.eu/downloads/latest/manual/en/w342_cs_cj_cp_nsj_communications_commands_reference_manual_en.pdf?v=1)

The run recorded 8,706 successful `0104` responses. The log contained no
`0102` write requests. NBDownload successfully restored the supplied
`newhmi.pkg`, the panel was reachable again, and requests to the emulator
stopped. Both transfers used the user project data option with the data
clearing options disabled.

For the remaining write trial, check that the initial state shows CIO120.03
off and CIO120.04 on.
Confirm a toggle and check the FINS log for the write and readback. Verify
that the second screen shows the same value. Cancel the next toggle and
check that no write was logged. Confirm a final toggle to restore the
starting value, then verify the restored state and the unchanged neighboring
bit. Restore the original user project after the trial.
