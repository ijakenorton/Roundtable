#include "rtaudio_c.h"
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// Platform-dependent sleep
#if defined(WIN32)
#include <windows.h>
#define SLEEP(ms) Sleep((DWORD)ms)
#else
#include <unistd.h>
#define SLEEP(ms) usleep((unsigned long)(ms * 1000.0))
#endif

// WAV file header structure
typedef struct {
    char chunkId[4];
    uint32_t chunkSize;
    char format[4];
    char subchunk1Id[4];
    uint32_t subchunk1Size;
    uint16_t audioFormat;
    uint16_t numChannels;
    uint32_t sampleRate;
    uint32_t byteRate;
    uint16_t blockAlign;
    uint16_t bitsPerSample;
    char subchunk2Id[4];
    uint32_t subchunk2Size;
} WavHeader;

void writeWavHeader(FILE *file, unsigned int channels, unsigned int sampleRate,
                    unsigned int bitsPerSample, unsigned long totalFrames) {
    WavHeader header;
    unsigned long dataSize = totalFrames * channels * (bitsPerSample / 8);

    memcpy(header.chunkId, "RIFF", 4);
    header.chunkSize = 36 + dataSize;
    memcpy(header.format, "WAVE", 4);
    memcpy(header.subchunk1Id, "fmt ", 4);
    header.subchunk1Size = 16;
    header.audioFormat = 1; // PCM
    header.numChannels = channels;
    header.sampleRate = sampleRate;
    header.byteRate = sampleRate * channels * (bitsPerSample / 8);
    header.blockAlign = channels * (bitsPerSample / 8);
    header.bitsPerSample = bitsPerSample;
    memcpy(header.subchunk2Id, "data", 4);
    header.subchunk2Size = dataSize;

    fwrite(&header, sizeof(WavHeader), 1, file);
}

typedef int16_t sample_t;
#define FORMAT RTAUDIO_FORMAT_SINT16
#define BITS_PER_SAMPLE 16

typedef struct {
    sample_t *buffer;
    unsigned long bufferBytes;
    unsigned long totalFrames;
    unsigned long frameCounter;
    unsigned int channels;
} InputData;

int input_callback(void *outputBuffer, void *inputBuffer,
                   unsigned int nBufferFrames, double streamTime,
                   rtaudio_stream_status_t status, void *userData) {
    InputData *data = (InputData *)userData;

    // Calculate how many frames to copy
    unsigned int frames = nBufferFrames;
    if (data->frameCounter + nBufferFrames > data->totalFrames) {
        frames = data->totalFrames - data->frameCounter;
        data->bufferBytes = frames * data->channels * sizeof(sample_t);
    }

    // Copy data to our buffer
    unsigned long offset = data->frameCounter * data->channels;
    memcpy(data->buffer + offset, inputBuffer, data->bufferBytes);
    data->frameCounter += frames;

    // Return 2 to stop the stream when done
    if (data->frameCounter >= data->totalFrames)
        return 2;
    return 0;
}

void usage(void) {
    printf("\nusage: record N fs <duration> <device> <channelOffset>\n");
    printf("    N          = number of channels\n");
    printf("    fs         = sample rate\n");
    printf("    duration   = optional time in seconds (default = 2.0)\n");
    printf("    device     = optional device index (default = 0)\n");
    printf("    channelOffset = optional channel offset (default = 0)\n\n");
    exit(0);
}

int main(int argc, char *argv[]) {
    unsigned int channels, fs, bufferFrames = 512;
    double time = 2.0;
    FILE *fd = NULL;
    InputData data = {0};
    rtaudio_t audio = NULL;

    // Parse command line
    if (argc < 3 || argc > 6)
        usage();

    channels = atoi(argv[1]);
    fs = atoi(argv[2]);
    if (argc > 3)
        time = atof(argv[3]);

    // Create RtAudio instance
    audio = rtaudio_create(RTAUDIO_API_UNSPECIFIED);
    if (!audio) {
        printf("Failed to create RtAudio instance!\n");
        return 1;
    }

    // Check for devices
    int deviceCount = rtaudio_device_count(audio);
    if (deviceCount < 1) {
        printf("No audio devices found!\n");
        goto cleanup;
    }

    // Set up stream parameters
    rtaudio_stream_parameters_t inputParams;
    inputParams.device_id = rtaudio_get_default_input_device(audio);
    inputParams.num_channels = channels;
    inputParams.first_channel = 0;

    if (argc > 5)
        inputParams.first_channel = atoi(argv[5]);

    // Prepare data buffer
    data.totalFrames = (unsigned long)(fs * time);
    data.frameCounter = 0;
    data.channels = channels;
    data.bufferBytes = bufferFrames * channels * sizeof(sample_t);

    unsigned long totalBytes = data.totalFrames * channels * sizeof(sample_t);
    data.buffer = (sample_t *)malloc(totalBytes);
    if (!data.buffer) {
        printf("Memory allocation error!\n");
        goto cleanup;
    }

    // Open the stream
    rtaudio_error_t err = rtaudio_open_stream(
        audio,
        NULL,              // no output
        &inputParams,      // input parameters
        FORMAT,            // format
        fs,                // sample rate
        &bufferFrames,     // buffer frames
        input_callback,    // callback function
        &data,             // user data
        NULL,              // stream options
        NULL               // error callback
    );

    if (err != RTAUDIO_ERROR_NONE) {
        printf("Error opening stream: %s\n", rtaudio_error(audio));
        goto cleanup;
    }

    // Start the stream
    err = rtaudio_start_stream(audio);
    if (err != RTAUDIO_ERROR_NONE) {
        printf("Error starting stream: %s\n", rtaudio_error(audio));
        goto cleanup;
    }

    printf("\nRecording for %.1f seconds ... writing file 'record.wav' (buffer frames = %u)\n",
           time, bufferFrames);

    // Wait for recording to finish
    while (rtaudio_is_stream_running(audio)) {
        SLEEP(100);
    }

    // Write WAV file
    fd = fopen("record.wav", "wb");
    if (fd) {
        writeWavHeader(fd, channels, fs, BITS_PER_SAMPLE, data.totalFrames);
        fwrite(data.buffer, sizeof(sample_t), data.totalFrames * channels, fd);
        fclose(fd);
        printf("Recording complete! Wrote %lu frames to record.wav\n", data.totalFrames);
    } else {
        printf("Failed to open output file!\n");
    }

cleanup:
    if (audio && rtaudio_is_stream_open(audio))
        rtaudio_close_stream(audio);
    if (audio)
        rtaudio_destroy(audio);
    if (data.buffer)
        free(data.buffer);

    return 0;
}
