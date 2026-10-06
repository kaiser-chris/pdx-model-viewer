# PDX Model Viewer

A viewer for the 3D models of Victoria 3, Europa Universalis 5 and Crusader
Kings 3. Open an `.asset` file and it draws the entities the file defines, with
their meshes, textures, attached entities and animations. A mod is read
together with the game it is for, which is looked for through Steam.

It runs on Windows and Linux.

## Opening a file

Associate the `.asset` extension with `pdx-model-viewer` (on Windows, **Open
with → Choose another app → Always**) and double-click a file, or open one with
**File → Open...** (`Ctrl+O`) or by dropping it onto the window. **File →
Recent** lists the last ten files.

The meshes and textures of a file are looked for where the game looks for them,
from the game, mod or DLC the file belongs to, which the viewer works out from
where the file is. A file outside any game, such as one kept on its own with
its meshes and textures, is drawn as well as it can be from the files around
it. **File → Reload** (`F5`) reads a game's definitions again, for files that
changed since.

## Mods

A mod is read with the game it is for: the game's files first and the mod's
over them, so what the mod does not define still comes from the game. Which
game that is comes from the mod's own `metadata.json` or `descriptor.mod`, and
the game itself is looked for through Steam.

A mod that names no game, or whose game is not installed, is asked about: the
chooser lists the three games with where each is installed, and **Browse...**
points at a game kept outside Steam. What it is answered is kept, so the same
mod is not asked about again.

## Viewing

A file with one entity shows it at once; a file with several lists them, to
pick one from, with a search box and the arrow keys. **Details** shows how the
entity is put together: its mesh and each part of it, the entities it attaches
and where they hang from, and, where the entity is a portrait accessory, the
patterns and colour palettes it may be drawn with, which can be picked.
**Problems** lists what could not be read.

| In the viewport                       | Effect                                |
|---------------------------------------|---------------------------------------|
| Drag                                  | Turn the camera around the model      |
| Drag with the right or middle button  | Move the view along                   |
| Scroll                                | Move closer or further                |
| Double-click, or `Home`               | Back to the front view, on the middle |
| `T`                                   | Turn the model by itself              |

An entity whose mesh can play animations gets an **Animation** panel along the
bottom of the viewport, listing what the entity and the entities it attaches
can play, with a timeline over the one picked: **Play**, **Stop**, **Loop**,
and a clock that says which frame the timeline stands at. An entity attached
many times has its animations listed once, with a tick for every copy of it,
and a copy left unticked stands still while the rest play.

The models are drawn with a shader and lighting of the viewer's own, an
approximation of the games' look rather than their own shaders.

The window opens maximized the first time, and after that the way it was
closed. The window, the layout of the panels, the recent files, the export
folder and the game answered for a mod are kept in `%AppData%\pdx-model-viewer`
on Windows and in `~/.config/pdx-model-viewer` on Linux. **View → Reset
Layout** puts the panels back.

## Exporting

**File → Export...** (`Ctrl+E`) writes the entity on view out as PNG files: the
view in the viewport and the front, back, left, right, top and bottom of the
whole model, over a background colour of your choosing or a transparent one, at
a size up to 8192 pixels a side. One view is saved where the save dialog says;
several go to a folder you choose, each named `<entity>_<view>.png`.

## Not supported yet

- the games' own shaders and the lighting of their environments: the look is an
  approximation;
- the shade and the row of a colour palette, which the games pick at random and
  of which the viewer draws the first;
- the files of `pdxmesh` definitions that no entity of the file draws.

## Building

The viewer is written in Go and draws with [raylib](https://www.raylib.com/)
and [Dear ImGui](https://github.com/ocornut/imgui), which are compiled along
with it, so a C and C++ compiler is needed as well as Go. The files of the
games are read and drawn by
[pdx-asset-go](https://github.com/kaiser-chris/pdx-asset-go) and
[pdx-parser-go](https://github.com/kaiser-chris/pdx-parser-go), both pinned to
a released version in `go.mod`.

### Windows

1. Install [Go](https://go.dev/dl/), in the version `go.mod` asks for or newer.
2. Install a MinGW-w64 toolchain, for example
   [w64devkit](https://github.com/skeeto/w64devkit) or the one that comes with
   [MSYS2](https://www.msys2.org/), and put its `bin` folder on `PATH`.
3. Run `build.bat`. The executable lands in `bin\windows`.

### Linux

1. Install [Go](https://go.dev/dl/), in the version `go.mod` asks for or newer.
2. Install a compiler and the OpenGL, X11 and Wayland development files. On
   Debian and Ubuntu:

   ```bash
   sudo apt-get install build-essential libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev
   ```

3. Run `./build.sh`. The executable lands in `bin/linux`.

### Tests

| Command          | Effect |
|------------------|--------|
| `make test`      | The tests that need neither a GPU nor a game |
| `make uitest`    | Also drives the real application in a hidden window, clicking and typing through Dear ImGui; needs a display but shows nothing on it |
| `make gametest PDX_GAME_DIR=...` | Also reads every asset file of real installations and views a sample of their entities |

`PDX_GAME_DIR` takes several game folders, separated the way `PATH` is. With
`PDX_DUMP_DIR` set as well, the viewport of every entity viewed is written out
as a PNG file, which is how to see that the models look right without a window.
