# Workspace relocation F: to C: - 2026-11-19

Follow-up: the interrupted [local profile-state phase](omron-cj1-local-profile-state.md)
has now been finalized on C:. The checkpoint below describes the work as it stood
at migration time.

The user requested a complete move fromF: toC:. All user data were copied and
verified before source removal. Current active locations:

| Content | New location |
| --- | --- |
| Workspace, projects, research, caches, tools, parent Git repository | `C:/cx-proger` |
| Nautilus repository | `C:/cx-proger/nautilus` |
| CX-One installation media | `C:/cx` |
| Original root recovery report and migration proof | `C:/cx-migration-20261119` |

## Verified copy

The inventories contain81079 objects:73671 regular files,2 symbolic links and
7406 directories. Every regular file was compared by size and SHA256 between
source and destination:9521634034 bytes total. Both symlink targets were
preserved exactly as relative paths. Windows required an elevated helper for
those two links; PowerShell's absolute-link normalization was corrected using
os.symlink before successful final verification.

Robocopy included hidden files, both Git repositories, tools, all caches,
`.transfer`, all projects and all research. The23MB migration log package is
under `C:/cx-migration-20261119/logs`; verification.json records success and
verified-files.jsonl contains the per-file source/destination/length/hash proof.
Its SHA256 is:
`a1df3f722068ed6a72adf8368c23368c380636ee9967f08527eb68de8ea8ad30`.
Protected F: system directories were excluded from this user-data move.

## Active paths and validation

`Dev-Environment.ps1` derives paths from its own directory and now selects
C: automatically. Go root/GOPATH/GOMODCACHE/GOCACHE were observed under
C:/cx-proger. Portable Go1.26.10 and Git run from the new tree. The original
main-repository HEADd2553787 and all five in-progress profile-state changes
were preserved. FINS and compiler tests passed from the C: worktree with
CGO disabled and offline dependencies.

Install-Workspace.ps1 and Probe-CX-One.ps1 now derive their workspace paths,
rather than using stale E: literals. WINDOWS_READY.md and current handoff
entry points point to C:. The Nautilus origin was a local recovery bundle
onE:; it now points to the copied `C:/cx-proger/.transfer/nautilus.bundle`.
No remote repository was written or pushed.

Frozen research files and historical manifests were not rewritten. Their
original E:/F: path strings describe capture provenance; substitute the current
root when invoking replay scripts with explicit path arguments. Do not bulk
rewrite frozen evidence or infer that recorded paths still locate active data.

The desktop attach_worktree tool rejected this ordinary checkout with
"Choose a registered linked Git worktree." No new checkout/thread was created.
Open `C:/cx-proger` in Codex to use the new folder as the app's project root;
current tool work already runs explicitly from the C: repository.

## Source removal

Automatic approval review initially rejected recursive deletion of the old
source trees, requiring separate confirmation of irreversible removal. The
user then explicitly answered: "Да, удалить проверенные исходники с F:".
After rechecking the saved manifest and the exact resolved paths, all three
approved sources were removed: F:/cx-proger, F:/cx and the root recovery report.
No deletion workaround was used. Only protected system directories remain
on F:, with approximately9.00GiB free. C: had approximately5.13GiB free after
validation. The removal log is `C:/cx-migration-20261119/source-removal.json`;
`migration-status.json` records completion.

With the original paths absent, FINS and compiler tests passed again using
`go test -count=1 -buildvcs=false -mod=readonly -timeout 180s` on the two
packages. This reran tests without cached test results. The same five
uncommitted research changes remain on C:. The data move is complete;
opening C:/cx-proger in Codex remains the app-side folder selection.

## Research checkpoint retained across the move

The interrupted phase added an opt-in ProfileStateProbe to the isolated native
worker and captured local default/current version bytes[4,0,0] and local
IsExtendedProtect0 on both profiles afterSetPLC. This is local library state,
not a hardware unit-version/protection result. The compiler settings audit now
reports this retained observation separately; hardware qualification remains
false. Its five source/fixture changes remain uncommitted for completion of
that research stage. All10 Omron packages, Omron CLI and vet passed before the
move; the27-check native evidence verifier and four launcher negative controls
also passed. No PLC request was made in this phase or during migration.

Retained in-progress artifacts, now under C:/cx-proger/research:

- `cj1-oracle/profile-state-20261119-01`;
- `hardware/20261119-cj1-local-profile-state-01`.

Next: finalize that phase's documentation/frozen manifests and commit; then
continue actual target protection/service selection and CPU finalization.
The physical-isolation authorization for both PLCs and previously verified
mode-only/read-only hardware results are unchanged by the filesystem move.
