# Synology tools

A Synology NAS running DSM 7: storage health and capacity, drive SMART status, resource use,
and what is on disk through File Station. Ten tools only read; four change files (create a
folder, rename, copy or move, delete) and exist only behind `allowedPaths`, below. Each tool runs
fixed DSM API calls; no tool takes an API or method name, so what the model can ask the NAS
is exactly this table. Only `synology_delete_files` deletes. Share, file and folder names are set by NAS users and are
passed through verbatim, flagged as data, not instructions. Paths must be absolute, start
with a share (`/NAS/Movies`) and contain no `..` segment; anything else is rejected before a
request is sent. File and search pages hold 25 entries by default and at most 100. The search
and folder-size tools run a File Station background task, wait a few seconds at most, always
stop the task, and mark the result `partial` if the wait ran out.

> **The account is a NAS administrator, and its password is full control of the NAS.** DSM
> only serves storage information to administrators, and an administrator can change
> shares, users, backups and the iSCSI volumes behind a Kubernetes cluster. ARR-MCP limits
> itself to the calls below, but anyone who reads the configured password is not limited at
> all: keep it out of images, logs and chat, give it a long random value, and use an account
> that exists only for this. DSM's "No access" setting on a share is not a second fence,
> since an administrator can lift it.
>
> Error text is scrubbed of the password and session id in every common spelling, but only for secrets of 8 or more characters: a shorter password is not masked, so use a longer one.
>
> **The connection is plain HTTP.** The URL is `http://<nas>:5000`: the password and the
> session id cross the network unencrypted, so use this only on a LAN you trust and never
> over the internet or a shared network. There is deliberately no option to skip TLS
> verification or to add a CA.

| Tool | Access |
|---|---|
| `synology_system_info` — model, DSM version, uptime, CPU, memory, temperature | read |
| `synology_utilization` — CPU load, memory, network and disk I/O right now | read |
| `synology_update_status` — whether a DSM update is waiting (only asks) | read |
| `synology_storage_overview` — pools and volumes: status, total, used and free bytes | read |
| `synology_list_disks` — each drive's model, status, SMART status, temperature, uncorrectable sectors | read |
| `synology_list_shares` — the shared folders this account can see; paged | read |
| `synology_list_files` — one folder's entries with size, modified time and owner; paged, sorted, filtered | read |
| `synology_get_file_info` — one file or folder in detail | read |
| `synology_search_files` — names below a folder, as a bounded File Station search | read |
| `synology_dir_size` — total bytes, folders and files below a folder, as a bounded File Station task | read |
| `synology_create_folder` — one new folder inside an existing folder | write |
| `synology_rename` — one file or folder to a new name, in the same folder | write |
| `synology_copy_move` — copy or move into a folder; a move removes the sources and `overwrite` replaces files, so the tool is | destructive |
| `synology_delete_files` — delete up to 10 files or folders as a File Station task; treat as permanent | destructive |

## `allowedPaths`: the only limit on file changes

The DSM account is a NAS administrator. DSM's own share permissions do not limit it (an
administrator can lift a "No access" setting), so **the `allowedPaths` setting is the only
limit on which files these four tools can change**. It is a list of absolute DSM paths on
the instance:

```yaml
synology:
  - name: main
    url: http://192.168.10.30:5000
    username: your-dsm-admin
    password: ${SYNOLOGY_PASSWORD}
    allowedPaths: ["/NAS"]      # a share, or a folder such as /NAS/Scratch
```

- Unset or empty, every write tool refuses everything, with an error that says how to set it.
  There is no environment-variable form: use a config file.
- Entries must be absolute, contain no `..`, and not be just `/`; the config fails to load otherwise.
- Every path a write tool takes must be an allowed path or inside one, on a path-segment
  boundary: `/NAS2` and `/NASx` are not inside `/NAS`, and `/NAS/../kube` is rejected. Matching
  is exact, letter case included, so `/nas/x` does not match `/NAS`.
- Rename takes a single new name (no `/`, no `..`), so the entry stays in its folder. Copy and
  move check every source and the destination folder. A source or rename target may not be an
  allowed path itself (so the fence root cannot be moved or renamed away); a destination may.
- The check runs before login and before any request. A refused call sends nothing to the NAS.
- **Links.** A name can be a link: `/NAS/x -> /volume1/kube` is textually inside `/NAS`. After the
  name check, and before anything is changed, the tools read the share's real path
  (`list_share`) and ask DSM (File Station `getinfo`, a read) about the allowed path, every
  existing path the call touches (the parent folder, the entry to rename and its parent, every
  copy/move source and the destination) and the entry the write will land on (the new folder or
  new name; for copy and move, the destination plus each source's name). Observed on DSM 7.4.1 in
  a live test on 2026-10-11: a symbolic link is hidden from folder listings and `getinfo`
  answers it with only `{"code":407}` (no `real_path`, no type), whether it is absolute or
  relative, points to a folder or a file, inside the share or outside it, or dangles. A path
  through a link gives 407 for an existing child but 408 (missing) for a missing child, so a
  missing landing entry is only trusted because its parent was itself checked and found plain.
  **What actually refuses a link is therefore the rule that any answer other than success (or
  "missing" for a landing entry) refuses the call.** The exact real-path comparison against the
  share's real path, and the checks on a mount point or link type, are a backstop that has never
  been seen to trip on this NAS. A refused call reads `could not check "<path>" for links: ...
  operation not permitted (code 407); nothing was changed`. In the live test a write was refused
  for create under a link, copy into a link, copy from a link and from a child of one, rename of
  a link, a landing entry that is a link (create, rename, copy and move, with overwrite false and
  true) and a link as the `allowedPaths` root; the target share was unchanged. This was observed
  on one NAS and one DSM version. Still open: this is check-then-act, so a link swapped in
  between the check and the write is not caught, and links nested inside a folder being copied
  or moved were not tested and are handled as DSM sees fit. `allowedPaths` guards against
  mistakes and planted links; it is not a sandbox against someone with write access who races
  the tool. A case-only rename is refused (the target "exists").
- `overwrite` defaults to false: DSM then leaves files that already exist in the destination
  untouched and does not report which; for a move, the source of a skipped file stays where it
  was (observed), so check with `synology_list_files`. A copy or move runs as a File Station
  task for up to about two minutes and is stopped after; `partial: true` means it was cut off
  and some files may be done. It is never started twice. The result carries no byte counts: DSM
  reported 0 for them on every copy and move, including ones that copied data. Observed after
  copies and moves: copied files are owned by the DSM account, and copied or moved files had
  mode 777 afterwards (a moved file kept its owner but went from 644 to 777).

- **Delete** (`synology_delete_files`) is the most restricted. Up to 10 paths per call, each listed once.
  The fence is strict: an `allowedPaths` entry itself can never be deleted, only things below it. A
  share root (`/NAS`) is always refused, allowlisted or not, and so is anything in a `#recycle` folder
  (the match ignores letter case). Every path must exist and pass the link check above, with no
  exception for a missing entry (a landing entry may be missing for a copy; a path to delete may not),
  and all of that runs for the whole batch before the one start request: one refusal fails the call
  and nothing is sent. It uses `SYNO.FileStation.Delete` v2 as a task (start, poll status, stop); the
  blocking `delete` method is not used. The start is never sent twice: an ambiguous start (transport
  error, no task id) is an error that says to check with `synology_list_files`.
  - **`recursive`.** The File Station API guide (2023.03 revision, Delete section) gives `recursive` a
    default of **true**, so the tool sends it explicitly on every start: `false` unless the caller passes
    `recursive: true`. The guide says that with `false` an error occurs on a folder that contains
    anything. **Observed on DSM 7.4.1, DSM does not error:** it accepts the start, reports the task
    finished with 0 of 0 processed, and leaves the folder and its contents in place. So the tool does not
    trust `finished`: after DSM reports it, one read-only `getinfo` over all the paths asked decides the
    outcome (per-entry code 408, no such file, means deleted). If anything is still there the call
    returns an **error** naming what was deleted and what remains, and saying a folder that contains
    anything is not deleted unless `recursive` is true. If that check itself fails, the error says DSM
    reported the task finished but the outcome could not be confirmed. The task is never started again.
  - **Recycle bin (observed on DSM 7.4.1, one NAS).** On a share with the recycle bin enabled (`NAS`),
    deleted files are moved to that share's `#recycle` folder, mirroring their path
    (`/NAS/x/f.txt` to `/NAS/#recycle/x/f.txt`), whole trees with their structure; links and empty
    folders are removed and not kept there. Other shares have no bin, so a delete there is permanent.
    Treat a delete as permanent unless you know the share has the bin on. The tool never deletes from
    `#recycle`, so it cannot empty the bin.
  - **Links inside a recursive delete.** Observed: in a folder deleted with `recursive: true`, a relative
    link to a folder, a link to a file and an absolute link were removed and their targets survived. The
    tool itself still cannot see or check links inside a folder (they are hidden from listings).
  - The result lists `deleted` (paths DSM then answered with code 408) and `remaining` (every other
    asked path), always as arrays, plus `finished` and `partial`. DSM's own `processed_num` and `total`
    are not reported: they were 0 and 0 for deletes that worked, so they cannot tell a no-op from a
    success. `partial: true` means the wait (about two minutes) ran out and the task was stopped; some
    entries may already be gone, and `deleted` and `remaining` are filled from a best-effort check.

The write tools are hidden when the instance's permission mode is `readonly`, and they prompt
in `confirm` mode like any other write tool.

Not here yet: deleting a share snapshot, running a Hyper Backup task and taking a share snapshot
are deferred: the NAS this was built against answers error 103 for the backup API and has no
snapshot to list, so no response shape could be verified. Hyper Backup task status and share
snapshots were also left out of the read tools for the same reason.
