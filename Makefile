WINLIBS :=  -lole32 -lwinmm -lksuser -lmfplat -lmfuuid -lwmcodecdspuuid
AUDIO_DIR := audio
MINGW := x86_64-w64-mingw32-g++
MINGW_CC := x86_64-w64-mingw32-gcc

.PHONY: windows linux clean cross-windows wsl

windows: microphoneWindows listAudioDevicesWindows
linux: microphone listAudioDevices microphone-c
wsl: microphone-c-pulse
cross-windows: microphone-c.exe

# C version using rtaudio_c API (Jack for native Linux)
microphone-c: $(AUDIO_DIR)/microphone.c
	gcc -o microphone-c $(AUDIO_DIR)/microphone.c $(AUDIO_DIR)/rtaudio_c.cpp $(AUDIO_DIR)/RtAudio.cpp \
		-D__UNIX_JACK__ -lpthread -lm -ljack -lstdc++

# Cross-compile C version for Windows from Linux/WSL
microphone-c.exe: $(AUDIO_DIR)/microphone.c
	$(MINGW_CC) -o microphone-c.exe $(AUDIO_DIR)/microphone.c $(AUDIO_DIR)/rtaudio_c.cpp $(AUDIO_DIR)/RtAudio.cpp \
		-D__WINDOWS_WASAPI__ -lpthread $(WINLIBS) -lstdc++ -static

# Original C++ versions
microphoneWindows: $(AUDIO_DIR)/microphone.cpp
	g++ -o microphoneWindows $(AUDIO_DIR)/microphone.cpp $(AUDIO_DIR)/RtAudio.cpp -D__WINDOWS_WASAPI__ $(WINLIBS)

listAudioDevicesWindows: $(AUDIO_DIR)/listAudioDevices.cpp
	g++ -o listAudioDevicesWindows $(AUDIO_DIR)/listAudioDevices.cpp $(AUDIO_DIR)/RtAudio.cpp -D__WINDOWS_WASAPI__ $(WINLIBS)

microphone: $(AUDIO_DIR)/microphone.cpp
	g++ -o microphone $(AUDIO_DIR)/microphone.cpp $(AUDIO_DIR)/RtAudio.cpp -D__UNIX_JACK__ -lpthread -lm -ljack

listAudioDevices: $(AUDIO_DIR)/listAudioDevices.cpp
	g++ -o listAudioDevices $(AUDIO_DIR)/listAudioDevices.cpp $(AUDIO_DIR)/RtAudio.cpp -D__UNIX_JACK__ -lpthread -ljack

listAudioDevices.exe: $(AUDIO_DIR)/listAudioDevices.cpp
	$(MINGW) -o listAudioDevices.exe $(AUDIO_DIR)/listAudioDevices.cpp $(AUDIO_DIR)/RtAudio.cpp -D__WINDOWS_WASAPI__ -static -lpthread $(WINLIBS)
clean:
	rm -f microphone microphone-c microphone-c-pulse microphone-c.exe \
		microphoneWindows listAudioDevices listAudioDevicesWindows *.wav
