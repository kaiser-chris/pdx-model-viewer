// A laptop with a graphics chip of its own besides the one in its processor
// runs a program on the slower, built in one unless the program asks for the
// other, which can draw a large model too slowly to be worked with. The
// drivers of NVIDIA Optimus and AMD switchable graphics look for these two
// variables exported from the executable, and run it on the dedicated chip
// when they are set.
__declspec(dllexport) unsigned long NvOptimusEnablement = 1;
__declspec(dllexport) int AmdPowerXpressRequestHighPerformance = 1;
