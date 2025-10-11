WINLIBS :=  -lole32 -lwinmm -lksuser -lmfplat -lmfuuid -lwmcodecdspuuid

microphoneWindows: microphone.cpp
        g++ -o microphoneWindows microphone.cpp RtAudio.cpp -D__WINDOWS_WASAPI__ $(WINLIBS)

listAudioDevicesWindows: listAudioDevices.cpp
        g++ -o listAudioDevicesWindows listAudioDevices.cpp RtAudio.cpp -D__WINDOWS_WASAPI__ $(WINLIBS)

microphone: microphone.cpp
        g++ -o microphone microphone.cpp RtAudio.cpp -D__UNIX_JACK__ -lpthread -lm -ljack 

listAudioDevices: listAudioDevices.cpp
        g++ -o listAudioDevices listAudioDevices.cpp RtAudio.cpp -D__UNIX_JACK__ -lpthread -ljack
