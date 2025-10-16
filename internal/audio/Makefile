WINLIBS :=  -lole32 -lwinmm -lksuser -lmfplat -lmfuuid -lwmcodecdspuuid
MINGW := x86_64-w64-mingw32-g++
MINGW_CC := x86_64-w64-mingw32-gcc

.PHONY: windows linux

windows: rtaudio_go_windows.o
linux: rtaudio_go_linux.o

rtaudio_go_windows.o:
	g++ -c -o rtaudio_go.o rtaudio_go.cpp -D__WINDOWS_WASAPI__ $(WINLIBS)

rtaudio_go_linux.o:
	g++ -c -o rtaudio_go.o rtaudio_go.cpp -D__UNIX_JACK__ -lpthread -lm -ljack -lstdc++
