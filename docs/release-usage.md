# WhichWhy

Which command gets picked — and why?

Extract the archive into a directory you choose. Run `whichwhy.exe` on Windows,
or `./whichwhy` on Linux/macOS. WhichWhy does not install itself, edit PATH, or
update itself. You may move the binary or add its directory to PATH yourself.

Start with `--version`, `--help`, `whichwhy python`, `whichwhy path`, and
`whichwhy doctor`. Add `--json` to inspection, path, or doctor for structured
evidence. Doctor can report attention when this directory is not on PATH.

Standalone inspection uses a named process PATH policy; it does not observe
your shell's aliases, functions or caches, or promise successful execution.
Inspection is local and passive: discovered commands are not executed.

In Windows PowerShell 5.1 or 7, optionally load the experimental bridge:

```powershell
.\whichwhy.exe init powershell | Invoke-Expression
whichwhy where
whichwhy where --json
```

This defines a function only in the current session. Discovery observes that
loaded session, does not auto-import unloaded modules, and retains limitations.
Regenerate the bridge after moving or upgrading the binary.

Full documentation: https://github.com/adhamcodes/whichwhy#readme
License: MIT; see the included LICENSE.
