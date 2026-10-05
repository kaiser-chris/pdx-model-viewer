package gui

// The Windows resources of the application, compiled from resources.rc into
// rsrc_windows_amd64.syso, which Go links into every Windows binary that uses
// this package.
//
// They carry the application icon, which Explorer also shows for the .asset
// files associated with the application, and the application manifest, whose
// one setting that matters is the heap. Windows gives applications installed
// from the Store the segment heap, which places large blocks at the very end
// of their memory pages, so reading even a byte past one crashes at once. The
// ordinary heap leaves some memory there, and the same bug goes unnoticed.
// With the segment heap in every build, a bug like that crashes in
// development and in the tests instead of only for the users of one build.
//
// After changing icon.ico, windows.manifest or resources.rc, regenerate the
// .syso with MinGW's windres:

//go:generate windres -i resources.rc -O coff -o rsrc_windows_amd64.syso
