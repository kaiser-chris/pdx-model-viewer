package workspace

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kaiser-chris/pdx-parser-go/asset"
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"

	"github.com/kaiser-chris/pdx-asset-go/entity"
	"github.com/kaiser-chris/pdx-asset-go/model"
	"github.com/kaiser-chris/pdx-asset-go/render"
	"github.com/kaiser-chris/pdx-asset-go/texture"
)

// Listing is what one asset file defines.
type Listing struct {
	// Entities and Meshes are the names of the entities and the pdxmesh
	// definitions of the file, in order of name.
	Entities []string
	Meshes   []string

	// Diagnostics are the problems with the file's syntax.
	Diagnostics report.Diagnostics
}

// List reads the definitions of the asset file at a location, and nothing
// else, which is quick enough to show them while the rest of the game is still
// being read.
func List(location Location) Listing {
	file := folders.File{Relative: location.Relative, Path: location.File, Source: SourceName(location.Root)}

	syntax := &report.Collector{}
	parsed := database.ParseFiles([]folders.File{file}, syntax)

	// What Read says about references is about this file on its own, which
	// is not how the game reads it: an entity whose mesh is defined in another
	// file is reported. Only the syntax counts here; the references are
	// checked against the whole game when an entity is loaded.
	defined := asset.Read(parsed[0].Document, file, &report.Collector{})

	return Listing{
		Entities:    sortedNames(defined.Entities.Keys()),
		Meshes:      sortedNames(defined.Meshes.Keys()),
		Diagnostics: syntax.Diagnostics,
	}
}

func sortedNames(names []string) []string {
	return slices.SortedFunc(slices.Values(names), func(a, b string) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a), strings.ToLower(b)), cmp.Compare(a, b))
	})
}

// SourceName is how a root is named to the user: the game's install folder
// for a game folder, such as Victoria 3, and the folder itself otherwise.
func SourceName(root string) string {
	name := filepath.Base(root)

	if strings.EqualFold(name, "game") {
		if install := filepath.Base(filepath.Dir(root)); install != "" && install != "." && install != string(filepath.Separator) {
			return install
		}
	}

	return name
}

// Game is the asset definitions of one game or mod, read from its root, and
// the loader that builds models from them.
//
// A Game is safe for use by several goroutines: loads run one at a time.
type Game struct {
	// Root is the folder the files were read from, and Name what it is called.
	Root string
	Name string

	// Diagnostics are the problems with the folder itself, such as one that
	// holds no game files.
	Diagnostics report.Diagnostics

	// Took is how long reading the definitions took.
	Took time.Duration

	// set is the game's folders, which a loose file has none of.
	set    *folders.Set
	assets *asset.Assets

	mu     sync.Mutex
	loader *entity.Loader
}

// OpenGame reads every asset definition below a root. It takes a second or
// so for a whole game.
//
// The folders of the game's engine, clausewitz and jomini next to the game
// folder of an installation, are read before the root, the way the engine
// mounts them: some of the textures are theirs.
func OpenGame(root string) (*Game, error) {
	start := time.Now()
	name := SourceName(root)

	own := folders.Open([]folders.Source{{Name: name, Path: root}})
	if len(own.Names()) == 0 {
		return nil, fmt.Errorf("%s holds no game or mod files", root)
	}

	set := folders.Open(append(entity.EngineFolders(root), folders.Source{Name: name, Path: root}))

	assets := asset.Load(set)

	game := &Game{
		Root:   root,
		Name:   name,
		set:    set,
		assets: assets,
		loader: entity.NewLoader(set, assets),
	}

	game.Diagnostics = append(game.Diagnostics, set.Diagnostics...)
	game.Took = time.Since(start)

	return game, nil
}

// OpenLoose reads a loose asset file, one of no game, on its own: the
// entities it defines are drawn with what is around it. The files they name
// are looked for where the file says, and by their names next to it; a
// texture of colour that is not there anyway shows as a checkerboard, a mesh
// that is not there as nothing.
func OpenLoose(location Location) (*Game, error) {
	start := time.Now()
	name := SourceName(location.Root)

	file := folders.File{Relative: location.Relative, Path: location.File, Source: name}

	syntax := &report.Collector{}
	parsed := database.ParseFiles([]folders.File{file}, syntax)
	if len(parsed) == 0 {
		return nil, fmt.Errorf("%s could not be read", location.File)
	}

	assets := asset.Read(parsed[0].Document, file, syntax)

	loader := entity.NewLoader(entity.Folder{Path: location.Root, Name: name}, assets)
	loader.MissingTexture = texture.Checkerboard()
	loader.EmptyWithoutMesh = true
	loader.ByName = true

	return &Game{
		Root:        location.Root,
		Name:        name,
		Diagnostics: syntax.Diagnostics,
		Took:        time.Since(start),
		assets:      assets,
		loader:      loader,
	}, nil
}

// Loose reports whether the game is a loose file's, of no game.
func (g *Game) Loose() bool {
	return g.set == nil
}

// Entities is how many entities the game defines.
func (g *Game) Entities() int {
	return g.assets.Entities.Len()
}

// Loaded is an entity built into a model, and what is worth knowing about it.
type Loaded struct {
	Model   *model.Model
	Details Details

	// Diagnostics are what could not be read and was drawn as well as it
	// could be, such as a texture that is missing.
	Diagnostics report.Diagnostics
}

// Details describe an entity the way its files put it together.
type Details struct {
	Entity string

	// Defined is where the entity is defined, as a path and a line.
	Defined string

	// Clones are the entities this one copies, nearest first.
	Clones []string

	// Mesh is the pdxmesh the entity draws, and MeshFile the file that
	// pdxmesh names, below the game's root.
	Mesh     string
	MeshFile string

	Parts []PartDetails
}

// PartDetails describe one part of a model.
type PartDetails struct {
	Name   string
	Shader string

	// Style is how the viewer draws the part, which it tells from the
	// shader's name.
	Style render.Style

	Vertices, Triangles int

	// Which of its textures were found. One that was not is drawn with a
	// neutral stand in and listed among the diagnostics.
	Diffuse, Normal, Properties bool

	// Drawn is false for a shape the game does not draw, which is left out
	// of the model.
	Drawn bool
}

// Load builds an entity of the game into a model.
func (g *Game) Load(name string) (*Loaded, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	built, diagnostics, err := g.loader.Load(name)
	if err != nil {
		return nil, err
	}

	details := g.describe(name, built)

	// What is left out is left out of the box the camera frames as well, or
	// a large collision shape would leave the model a speck in the middle.
	built.Parts = slices.DeleteFunc(built.Parts, func(part model.Part) bool { return !drawn(part) })
	built.Bounds()

	return &Loaded{Model: built, Details: details, Diagnostics: diagnostics}, nil
}

// drawn reports whether the game draws a part: one its mesh settings say how
// to draw. A shape without any, such as the collision shape some buildings
// carry, is there for the engine rather than to be seen.
func drawn(part model.Part) bool {
	textures := part.Textures

	return part.Shader != "" || textures.Diffuse != nil || textures.Normal != nil || textures.Properties != nil
}

// Portrait reports whether an entity is drawn like a portrait's skin, which
// is what the palette colour of a viewer stands for there: the skin tone.
func (d Details) Portrait() bool {
	return slices.ContainsFunc(d.Parts, func(part PartDetails) bool {
		return strings.HasPrefix(part.Shader, "portrait") && strings.Contains(part.Shader, "skin")
	})
}

func (g *Game) describe(name string, built *model.Model) Details {
	details := Details{Entity: name}

	for index, entity := range g.assets.CloneChain(name) {
		if index == 0 {
			origin := entity.Origin()
			details.Defined = fmt.Sprintf("%s:%d", origin.File, origin.Line)
		} else {
			details.Clones = append(details.Clones, entity.Key)
		}
	}

	if mesh, ok := g.assets.MeshOf(name); ok {
		details.Mesh = mesh.Key
		details.MeshFile = asset.Resolve(mesh.Origin(), mesh.File)
	}

	for _, part := range built.Parts {
		described := PartDetails{
			Name:       part.Name,
			Shader:     part.Shader,
			Style:      render.StyleOf(&part),
			Diffuse:    part.Textures.Diffuse != nil,
			Normal:     part.Textures.Normal != nil,
			Properties: part.Textures.Properties != nil,
			Drawn:      drawn(part),
		}

		for index := range part.Pieces {
			described.Vertices += part.Pieces[index].Vertices()
			described.Triangles += part.Pieces[index].Triangles()
		}

		details.Parts = append(details.Parts, described)
	}

	return details
}
