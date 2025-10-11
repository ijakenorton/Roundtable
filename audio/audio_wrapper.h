#ifndef AUDIO_WRAPPER_H
#define AUDIO_WRAPPER_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque handle for audio device
typedef struct audio_device audio_device_t;

// Error codes
#define AUDIO_OK 0
#define AUDIO_ERROR -1
#define AUDIO_NO_DEVICES -2
#define AUDIO_INVALID_PARAM -3

// Create an audio device instance
audio_device_t* audio_create();

// Destroy audio device and free resources
void audio_destroy(audio_device_t* dev);

// Get number of available audio devices
int audio_device_count(audio_device_t* dev);

// Get default input device ID
unsigned int audio_get_default_input(audio_device_t* dev);

// Get default output device ID
unsigned int audio_get_default_output(audio_device_t* dev);

// Start recording from input device
// Returns actual buffer size or negative error code
int audio_start_recording(audio_device_t* dev, unsigned int deviceId,
                          unsigned int channels, unsigned int sampleRate,
                          unsigned int bufferFrames);

// Stop recording/playback
void audio_stop(audio_device_t* dev);

// Check if stream is running
int audio_is_running(audio_device_t* dev);

// Read available audio samples (non-blocking)
// Returns number of frames read, or negative error code
int audio_read_samples(audio_device_t* dev, int16_t* buffer, unsigned int maxFrames);

// Get last error message
const char* audio_error_message(audio_device_t* dev);

#ifdef __cplusplus
}
#endif

//=============================================================================
// IMPLEMENTATION
//=============================================================================

#ifdef AUDIO_WRAPPER_IMPLEMENTATION

#include "rtaudio_c.h"
#include <stdlib.h>
#include <string.h>

#define RING_BUFFER_SIZE 48000 * 10  // 10 seconds at 48kHz

typedef struct {
    int16_t data[RING_BUFFER_SIZE];
    unsigned int writePos;
    unsigned int readPos;
    unsigned int available;
} ring_buffer_t;

struct audio_device {
    rtaudio_t handle;
    ring_buffer_t ringBuffer;
    unsigned int channels;
    char errorMsg[256];
};

// Ring buffer helpers
static void ring_buffer_init(ring_buffer_t* rb) {
    rb->writePos = 0;
    rb->readPos = 0;
    rb->available = 0;
}

static unsigned int ring_buffer_write(ring_buffer_t* rb, const int16_t* data,
                                     unsigned int samples) {
    unsigned int written = 0;
    while (written < samples && rb->available < RING_BUFFER_SIZE) {
        rb->data[rb->writePos] = data[written];
        rb->writePos = (rb->writePos + 1) % RING_BUFFER_SIZE;
        rb->available++;
        written++;
    }
    return written;
}

static unsigned int ring_buffer_read(ring_buffer_t* rb, int16_t* data,
                                    unsigned int samples) {
    unsigned int read = 0;
    while (read < samples && rb->available > 0) {
        data[read] = rb->data[rb->readPos];
        rb->readPos = (rb->readPos + 1) % RING_BUFFER_SIZE;
        rb->available--;
        read++;
    }
    return read;
}

// RtAudio callback - called from audio thread
static int audio_callback(void* outputBuffer, void* inputBuffer,
                         unsigned int nFrames, double streamTime,
                         rtaudio_stream_status_t status, void* userData) {
    audio_device_t* dev = (audio_device_t*)userData;

    if (inputBuffer) {
        int16_t* samples = (int16_t*)inputBuffer;
        unsigned int totalSamples = nFrames * dev->channels;
        ring_buffer_write(&dev->ringBuffer, samples, totalSamples);
    }

    return 0;  // Continue stream
}

audio_device_t* audio_create() {
    audio_device_t* dev = (audio_device_t*)malloc(sizeof(audio_device_t));
    if (!dev) return NULL;

    dev->handle = rtaudio_create(RTAUDIO_API_UNSPECIFIED);
    if (!dev->handle) {
        free(dev);
        return NULL;
    }

    ring_buffer_init(&dev->ringBuffer);
    dev->channels = 0;
    dev->errorMsg[0] = '\0';

    return dev;
}

void audio_destroy(audio_device_t* dev) {
    if (!dev) return;

    if (dev->handle) {
        if (rtaudio_is_stream_open(dev->handle)) {
            rtaudio_close_stream(dev->handle);
        }
        rtaudio_destroy(dev->handle);
    }

    free(dev);
}

int audio_device_count(audio_device_t* dev) {
    if (!dev || !dev->handle) return AUDIO_ERROR;
    return rtaudio_device_count(dev->handle);
}

unsigned int audio_get_default_input(audio_device_t* dev) {
    if (!dev || !dev->handle) return 0;
    return rtaudio_get_default_input_device(dev->handle);
}

unsigned int audio_get_default_output(audio_device_t* dev) {
    if (!dev || !dev->handle) return 0;
    return rtaudio_get_default_output_device(dev->handle);
}

int audio_start_recording(audio_device_t* dev, unsigned int deviceId,
                         unsigned int channels, unsigned int sampleRate,
                         unsigned int bufferFrames) {
    if (!dev || !dev->handle) return AUDIO_ERROR;
    if (channels == 0 || sampleRate == 0) return AUDIO_INVALID_PARAM;

    // Set up input parameters
    rtaudio_stream_parameters_t inputParams;
    inputParams.device_id = deviceId;
    inputParams.num_channels = channels;
    inputParams.first_channel = 0;

    dev->channels = channels;
    unsigned int frames = bufferFrames;

    // Open the stream
    rtaudio_error_t err = rtaudio_open_stream(
        dev->handle,
        NULL,                        // no output
        &inputParams,                // input parameters
        RTAUDIO_FORMAT_SINT16,       // 16-bit samples
        sampleRate,
        &frames,                     // buffer frames (may be modified)
        audio_callback,              // callback function
        dev,                         // user data
        NULL,                        // stream options
        NULL                         // error callback
    );

    if (err != RTAUDIO_ERROR_NONE) {
        snprintf(dev->errorMsg, sizeof(dev->errorMsg),
                "%s", rtaudio_error(dev->handle));
        return AUDIO_ERROR;
    }

    // Start the stream
    err = rtaudio_start_stream(dev->handle);
    if (err != RTAUDIO_ERROR_NONE) {
        snprintf(dev->errorMsg, sizeof(dev->errorMsg),
                "%s", rtaudio_error(dev->handle));
        rtaudio_close_stream(dev->handle);
        return AUDIO_ERROR;
    }

    return (int)frames;  // Return actual buffer size
}

void audio_stop(audio_device_t* dev) {
    if (!dev || !dev->handle) return;

    if (rtaudio_is_stream_running(dev->handle)) {
        rtaudio_stop_stream(dev->handle);
    }

    if (rtaudio_is_stream_open(dev->handle)) {
        rtaudio_close_stream(dev->handle);
    }
}

int audio_is_running(audio_device_t* dev) {
    if (!dev || !dev->handle) return 0;
    return rtaudio_is_stream_running(dev->handle);
}

int audio_read_samples(audio_device_t* dev, int16_t* buffer, unsigned int maxFrames) {
    if (!dev || !buffer || maxFrames == 0) return AUDIO_INVALID_PARAM;

    unsigned int totalSamples = maxFrames * dev->channels;
    unsigned int samplesRead = ring_buffer_read(&dev->ringBuffer, buffer, totalSamples);

    return samplesRead / dev->channels;  // Return frames, not samples
}

const char* audio_error_message(audio_device_t* dev) {
    if (!dev) return "Invalid device";
    return dev->errorMsg[0] ? dev->errorMsg : "No error";
}

#endif // AUDIO_WRAPPER_IMPLEMENTATION

#endif // AUDIO_WRAPPER_H
