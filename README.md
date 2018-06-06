# find-stignore

Tested on Linux and Windows.
Not particularly RAM efficient.

A simple tool that you compile and run on a syncthing host.
It only uses read-only API calls and local directory reads so is
non-destructive.

Two uses:
1) Validate that your ignore list is doing what you actually expect.
2) Using -print0, you could pipeline into something like xargs and rm if you want to...
(don't unless you know what you're doing...)

```
$ ./find-stignore --help
Usage of ./find-stignore:
  -apikey string
        The API key that the Syncthing gui shows. (Required)
  -dirsonly
        Only output directories, not files
  -filesonly
        Only output files, not directories
  -folderid string
        The folder ID that you want to check for ignores. See the Syncthing GUI. (Required)
  -print0
        Output in a format suitable for tools like xargs by null (zero) terminating lines. This means that files with newlines will be correctly listed.
  -showallconfig
        Show all config files (e.g. .stfolder, .stversions) Do NOT use the output with this enabled unless you are sure what you are looking at.
  -url string
        The host URL. Do not attempt to use this with a 'remote' Syncthing server since it expects to see the local filesystem that matches what syncthing sees. (default "http://localhost:8384")
```

## Installation

You'll need to download and then use go build to create an executable.
Or just download the "find-stignore" (linux x64) or "find-stignore.exe" (windows x64)

## Examples

No files are being ignored on this folder.

```
$ ./find-stignore --apikey xtEitme[snip]pn --folderid 6t6j[snip]g-fyb
$
```

Show the hidden config files that syncthing ignores.The reason for this - the files / folders are stil there, and are being ignored, but not by any ignore file. It at least proves that things are working when no other output is shown ;-)

```
$ ./find-stignore --apikey xtEitme[snip]pn --folderid 6t6j[snip]g-fyb --showallconfig
/media/syncthing/Laptop-Backup/.stfolder
/media/syncthing/Laptop-Backup/.stignore
$
```

Add a file that is being ignored, then re-run:

```
$ touch /media/syncthing/Laptop-Backup/Downloads/.DS_Store
$ ./find-stignore --apikey xtEitme[snip]pn --folderid 6t6j[snip]g-fyb
/media/syncthing/Laptop-Backup/Downloads/.DS_Store
$
```

And now re-runing using print0
```
$ ./find-stignore --apikey xtEitme[snip]pn --folderid 6t6j[snip]g-fyb -print0 | xargs -0 echo
/media/syncthing/Laptop-Backup/Downloads/.DS_Store
$
```

If you do decide to change that echo to an rm, then *please* run first with an echo, make sure the entire output is what you expect. I also *strongly* recommend you pause syncing the folder, and have a very recent backup.

Use --filesonly if you plan on doing rm, and --dirsonly if you plan on doing rmdirs.