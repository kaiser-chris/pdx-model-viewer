# PDX Model Viewer

A small viewer for the 3D models of Victoria 3, Europa Universalis 5 and
Crusader Kings 3. Open an `.asset` file and it shows the entities the file
defines, drawn with their meshes and textures.

It runs on Windows and Linux.

## Usage

### Opening a file

The viewer is not set up with a game folder. It is meant to be what `.asset`
files open with: associate the extension with `pdx-model-viewer` (on Windows,
**Open with → Choose another app → Always**) and double-click a file. It
opens with **File → Open...** (`Ctrl+O`) as well, and a file dropped onto the
window.

**File → Recent** lists the last ten files opened, to open one again, and the
open dialog starts in the folder of the last one.

The meshes and textures of the file are looked for where the game looks for
them, from the game the file belongs to, which the viewer works out from
where the file is:

- the folder above the `gfx` folder the file is in is the root of the game or
  mod, so `Victoria 3/game/gfx/models/.../x.asset` belongs to `Victoria 3/game`;
- a layer, such as Europa Universalis 5's `in_game`, belongs to the folder
  above it;
- a DLC, below the game's `dlc` folder, belongs to the game, so that what it
  uses from the game is found as well.

A mod that draws meshes or textures of the game it changes is not supported
yet: it is read on its own.

A file outside the `gfx` folder of any game or mod, such as one kept on its own
with its meshes and textures, is drawn as well as it can be, on its own:

- the meshes and textures it names are looked for where it says, relative to
  its folder, and by their file names next to it, so a file taken out of a
  game with the files it uses works;
- a texture of colour that is not there shows as a magenta and black
  checkerboard, a missing normal or properties map is left out, and a missing
  mesh, or one defined in another asset file, shows as nothing;
- an entity it attaches that it does not define is looked for in the asset
  files next to it, and one that is not there either is left out and listed
  under **Problems**.

The game's asset definitions are read once, which takes under a second, and
kept while the viewer runs, so the next file of the same game opens at once.
**File → Reload** (`F5`) reads them again, for files that changed since.

### Viewing

A file with one entity shows it at once; a file with several lists them on the
right, to pick one from. The arrow keys step through the list, and the search
box narrows it down.

| In the viewport                | Effect                          |
|--------------------------------|---------------------------------|
| Drag                           | Turn the camera around the model |
| Drag with the right or middle button | Move the view along      |
| Scroll                         | Move closer or further          |
| Double-click, or `Home`        | Back to the front view, on the middle of the model |
| `T`                            | Turn the model by itself        |

**Details** shows how the entity is put together: where it is defined, the
entities it clones, its mesh, and each part of the mesh in words: its name,
how it is drawn (solid, laid over the model like a decal, cut out like
leaves), which of its diffuse, normal and properties maps are missing, and its
triangles. The names the files give a part and its shader show when the
pointer rests on its name. Parts the game does not draw, such as collision
shapes, are listed as such and left out. **Problems** lists what could not be read, such as
a texture that was not found, which is drawn with a neutral stand in.

An entity is drawn with the entities it attaches, and those they attach,
each where it hangs: at a locator of the entity or of its mesh, or at a bone
of its mesh. An entity attaches entities by name, wherever in the game they
are defined, and some are nothing but what they attach, such as the hubs of
Victoria 3, which lay out a whole town. **Details** lists them under
**Attached**, each with the point it hangs from, opening onto its parts and
what it attaches in turn. One that is not there is marked, left out and
listed under **Problems**. Of a group of attachments the game picks from at
random, the first is shown.

An entity whose mesh can play animations gets an **Animation** panel, docked
along the bottom of the viewport. It lists what the entity and the entities it
attaches can play, with how long each runs, and runs a clock over a timeline
that reaches as far as the entity's longest animation, so the lengths can be
seen against one another. **Play** runs it, **Loop** starts it again at its
end, **Rewind** puts it back to the beginning, and the timeline can be dragged
to any moment of it. The clock says which frame that moment is, and the rate
the animation was made at.

The geometry is not moved by this yet: the panel reads the animations and runs
their clock, but the model is drawn in the pose its mesh file stores. Moving
it needs the mesh's joints and weights carried through to the GPU, which the
viewer does not do yet.

The models are drawn with a shader of the viewer's own, an approximation of
the games' look rather than their own shaders and lighting. It tells from the
name of each part's shader how to draw it: skin with the palette colour,
decals laid over the rest, leaves and hair cut out by their alpha.

The palette colour is what the games blend in where a diffuse map's alpha says,
such as a skin tone. It only shows on parts drawn as skin, so **Details**
offers it only for an entity that has some, as the skin tone of a portrait.
The viewer picks a skin tone for a portrait's skin and no tint for anything
else, until you pick one yourself.

Trees, whose leaves the games keep grey and colour as they draw them, are
coloured by the tint their files name, or a green of leaves where they name
none.

The viewer opens maximized the first time, and after that the way it was
closed: maximized or not, and at the size and place it had. The window, the
layout of the panels, the recent files and the export folder are kept in
`%AppData%\pdx-model-viewer` on Windows and in `~/.config/pdx-model-viewer`
on Linux. **View → Reset Layout** puts the panels back: the viewport on the
left, and on the right the entities above the details.

### Exporting

**File → Export...** (`Ctrl+E`) writes the entity on view out as PNG files:

- the views to export: the current view, as the viewport shows it, and the
  front, back, left and right sides, top and bottom, each of the whole model;
- a background colour, off to begin with, which leaves the background
  transparent;
- the size of the pictures, the desktop's resolution to begin with. **50%**,
  **Desktop** and **200%** set it from the desktop's resolution; a side can be
  up to 8192 pixels. The size set last is kept for the next run.

One view is saved where the save dialog says; several go to a folder you
choose. The dialogs start where the last export went. Either way a file is
named `<entity>_<view>.png`, such as `statue_entity_front.png`. Where a name
is taken, a number is added, and the views of one export share it, so that
they stay one group: `statue_entity_front_1.png`, `statue_entity_back_1.png`.

### Not supported yet

What pdx-asset-go does not draw yet, the viewer does not show either:

- skinning and animation: models are drawn in the pose their mesh files store,
  so a rider stands on the saddle rather than sitting in it;
- the games' own shaders and the lighting of their environments: the look is
  an approximation;
- the files of `pdxmesh` definitions that no entity of the file draws.

## Building

The viewer is written in Go and draws with [raylib](https://www.raylib.com/)
and [Dear ImGui](https://github.com/ocornut/imgui), which are compiled along
with it, so a C and C++ compiler is needed as well as Go. The files of the
games are read and drawn by
[pdx-asset-go](https://github.com/kaiser-chris/pdx-asset-go) and
[pdx-parser-go](https://github.com/kaiser-chris/pdx-parser-go).

Until pdx-asset-go is released, it is built from a checkout next to this one,
through a Go workspace:

```
go work init . ../pdx-asset-go
```

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
