package device

import (
	"context"
	"sync"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
)

// --------------------------------------------------------------------------------
// Fan Out Device (One to Many)

// A FanOutDevice is both an AudioSourceDevice and an AudioSinkDevice.
//
// Unlike other AudioSourceDevices, a call to GetStream does *not* return the
// singular output stream (sinkStream), but instead creates a *new* output stream unique to that call.
// Once added, there is no manual way to remove a sinkStream.
// A sinkStream is automatically removed if it does not accept a frame for a certain duration.
//
// The input stream (the sourceStream) is listened to and data forwarded (copied) to
// all sinkStreams. This may be an expensive process.
//
// Be sure to call SetStream before calls to GetStream to prevent the channels returned
// by GetStream from timing out before any data is ready to be received.
//
// Adding and removing sinkStreams is concurrency safe thanks to a mutex.
type FanOutDevice struct {
	deviceProperties audiodevice.DeviceProperties
	// A master context to cancel ALL sinks at once
	// sink channel contexts will be spawned as sub-contexts of this one.
	masterContext           context.Context
	masterContextCancelFunc context.CancelFunc

	sourceStream <-chan frame.PCMFrame

	sinksMutex sync.RWMutex
	sinks      []fanOutSink
}

type fanOutSink struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	stream    chan frame.PCMFrame
}

// Return a new sink context related to the fanOutDevice.masterContext
// such that the returned context has a fresh timeout, but is still canceled
// if the masterContext is canceled
func (d *FanOutDevice) newSinkContext() (context.Context, context.CancelFunc) {
	const SINK_TIMEOUT = 5 * time.Second
	return context.WithTimeout(d.masterContext, SINK_TIMEOUT)
}

// Create a new FanOutDevice.
// The given device properties are for book-keeping only.
// The properties do not influence the behavior of the FanOutDevice in any way.
func NewFanOutDevice(properties audiodevice.DeviceProperties) FanOutDevice {
	masterContext, masterContextCancelFunction := context.WithCancel(context.Background())
	return FanOutDevice{
		deviceProperties:        properties,
		masterContext:           masterContext,
		masterContextCancelFunc: masterContextCancelFunction,
		sinks:                   make([]fanOutSink, 0),
	}
}

func (d *FanOutDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return d.deviceProperties
}

// Set the stream of this device to copy data from.
// This method should be called only once, and once the sourceStream is closed
// then all sinkChannels are closed.
func (d *FanOutDevice) SetStream(sourceStream <-chan frame.PCMFrame) {
	d.sourceStream = sourceStream

	go func() {
		for data := range d.sourceStream {
			d.sinksMutex.Lock()
			// TODO: One channel blocking here will cause all channels to block.
			// Current select approach drops data to listeners who can't accept it... is that fine?
			for _, sink := range d.sinks {
				select {
				case sink.stream <- data:
					// We sent some data, let's refresh the sink context
					// First, cancel the old context
					sink.ctxCancel()
					// Then make a new one
					sink.ctx, sink.ctxCancel = d.newSinkContext()
				case <-sink.ctx.Done():
					// The sink didn't respond and has timed out, remove it
					close(sink.stream)
					numSinks := len(d.sinks)
					for i, s := range d.sinks {
						if s.stream == sink.stream {
							d.sinks[i] = d.sinks[numSinks-1]
							d.sinks = d.sinks[:numSinks-1]
							return
						}
					}
				default:
					// We couldn't send data, but the sink hasn't timed out, just move on
				}
			}
			d.sinksMutex.Unlock()
		}
		// When sourceStream closes, close this device
		d.Close()
	}()
}

// Get a new stream from this fan out device.
//
// This method returns a new stream that data from the sourceChannel is copied to.
// The returned channel must consume data as it arrives and is fanned out.
// If enough frames are rejected by the channel (e.g. because it is blocking)
// then the channel is closed. The close occurs with a timeout, set to 5 seconds.
//
// Be sure to call this method AFTER setStream, otherwise you risk
// the returned channel closing from a timeout before any data can be written to it!
func (d *FanOutDevice) GetStream() <-chan frame.PCMFrame {
	d.sinksMutex.Lock()
	defer d.sinksMutex.Unlock()

	sinkCtx, sinkCtxCancel := d.newSinkContext()
	newSink := fanOutSink{
		ctx:       sinkCtx,
		ctxCancel: sinkCtxCancel,
		stream:    make(chan frame.PCMFrame),
	}
	d.sinks = append(d.sinks, newSink)

	return newSink.stream
}

func (d *FanOutDevice) Close() {
	d.sinksMutex.Lock()
	defer d.sinksMutex.Unlock()
	d.masterContextCancelFunc()
	for _, sink := range d.sinks {
		sink.ctxCancel()
		close(sink.stream)
	}
	d.sinks = d.sinks[:0]
}

// --------------------------------------------------------------------------------
// Fan In Device (Many to One)

// A FanInDevice is both an AudioSourceDevice and an AudioSinkDevice.
//
// Unlike other AudioSinkDevices, a call to SetStream does *not* set the singular input stream
// but instead adds the given stream to a list of sourceStreams which are all checked for input
// and combined together to be returned along the sinkStream.
// A closed sourceStream is removed from the list, but does not close this device.
//
// The output stream (the sinkStream) gives the combined PCMFrames of the sourceStreams.
// Frames are not buffered, but read from the sourceStreams at once (streams with no data are skipped)
// and combined by simple addition (with clipping to values of +/- 1.0).
//
// A fan in device also requires a waitLatency, a duration to wait between listening for new frames.
// Setting this too low may result in poor mixing, with frames being sent as soon as they arrive, leading to
// choppy, overlapping audio.
// For safety, try leaving a few milliseconds for mixing. Something close close to the OPUS
// frame duration of the opposite peer would be ideal, but this may not be achievable.
type FanInDevice struct {
	deviceProperties audiodevice.DeviceProperties
	waitLatency      time.Duration

	masterContext           context.Context
	masterContextCancelFunc context.CancelFunc

	shutdownOnce sync.Once

	sourceStreamsMutex sync.RWMutex
	sourceStreams      []<-chan frame.PCMFrame

	sinkStream chan frame.PCMFrame
	sinkBuffer frame.PCMFrame
}

// Create a new FanInDevice.
// The given device properties are a promise: it is expected that all
// incoming frames will have EXACTLY this format. Therefore, consider using
// an AudioFormatConversionDevice before this device.
func NewFanInDevice(properties audiodevice.DeviceProperties, waitLatency time.Duration) *FanInDevice {
	masterContext, masterContextCancelFunction := context.WithCancel(context.Background())

	d := &FanInDevice{
		deviceProperties:        properties,
		waitLatency:             waitLatency,
		masterContext:           masterContext,
		masterContextCancelFunc: masterContextCancelFunction,
		sourceStreams:           make([]<-chan frame.PCMFrame, 0),
		sinkStream:              make(chan frame.PCMFrame),
		// The sink buffer should be large enough to hold PCM frames from any device.
		// It's incredibly unlikely that one full second of audio will ever arrive,
		// so leave enough room for this many samples.
		sinkBuffer: make(frame.PCMFrame, properties.SampleRate*properties.NumChannels),
	}
	d.startListening()

	return d
}

func (d *FanInDevice) startListening() {
	go func() {
		// Define the start index of the current output frame
		sinkBufferHead := 0
		var frame frame.PCMFrame
		listenTicker := time.NewTicker(d.waitLatency)
		defer listenTicker.Stop()
		for {
			select {
			case <-listenTicker.C:
			case <-d.masterContext.Done():
				return
			}

			d.sourceStreamsMutex.Lock()
			// Define the end index of the current output frame
			sinkBufferTail := sinkBufferHead
			for _, sourceStream := range d.sourceStreams {
				select {
				case frame = <-sourceStream:
				default:
					// If no frame is ready, skip this source stream
					continue
				}
				// We have a frame, process it

				// Truncate frame to size of buffer (just in case)
				// Keeping the *final* samples, not the first samples
				frame = frame[max(0, len(frame)-len(d.sinkBuffer)):]

				// First, check if we are about to overrun the end of the buffer
				if sinkBufferHead+len(frame) > len(d.sinkBuffer) {
					copy(d.sinkBuffer, d.sinkBuffer[sinkBufferHead:sinkBufferTail])
					sinkBufferTail = sinkBufferTail - sinkBufferHead
					sinkBufferHead = 0
				}

				// Now we know there must be space enough to put the frame into the buffer
				frameIndex := 0

				// For the sample indices that have been covered already (by frames from previous sourceStreams)
				// add the existing values and the new values
				existingDataLength := min(len(frame), sinkBufferTail-sinkBufferHead)
				for ; frameIndex < existingDataLength; frameIndex += 1 {
					d.sinkBuffer[sinkBufferHead+frameIndex] += frame[frameIndex]
				}
				// For the sample indices past the end of the existing frame buffer, set (rather than add)
				for ; frameIndex < len(frame); frameIndex += 1 {
					d.sinkBuffer[sinkBufferHead+frameIndex] = frame[frameIndex]
				}

				// Set the new buffer tail depending on whether we overran the existing buffer tail with this frame
				sinkBufferTail = max(sinkBufferTail, sinkBufferHead+frameIndex)
			}
			d.sourceStreamsMutex.Unlock()

			if sinkBufferHead == sinkBufferTail {
				continue
			}

			// We have read from every source!
			// Now the existing frame lives at d.sinkBuffer[sinkBufferHead:sinkBufferTail]
			// So perform a single clipping loop, then send, and update the tail

			for i := sinkBufferHead; i < sinkBufferTail; i += 1 {
				d.sinkBuffer[i] = max(-1.0, min(1.0, d.sinkBuffer[i]))
			}
			select {
			case <-d.masterContext.Done():
				return
			case d.sinkStream <- d.sinkBuffer[sinkBufferHead:sinkBufferTail]:
			default:
			}
			sinkBufferHead = sinkBufferTail
		}
		// This goroutine closes when the master context is cancelled,
		// which occurs when the Close function of this device is called.
	}()
}

func (d *FanInDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return d.deviceProperties
}

// Set a new stream of this device to receive data from.
//
// The given stream is read from and combined with all other streams set this way.
// When the given sourceStream is closed, it is removed from this device.
func (d *FanInDevice) SetStream(sourceStream <-chan frame.PCMFrame) {
	d.sourceStreamsMutex.Lock()
	defer d.sourceStreamsMutex.Unlock()
	d.sourceStreams = append(d.sourceStreams, sourceStream)
}

// Get the output of this FanInDevice.
//
// The returned stream combines data from all source streams
// simply adding these streams together and clipping as required.
func (d *FanInDevice) GetStream() <-chan frame.PCMFrame {
	return d.sinkStream
}

// Close this device.
// Stop listening on the sourceStreams, and close the sinkStream
func (d *FanInDevice) Close() {
	d.shutdownOnce.Do(func() {
		d.sourceStreamsMutex.Lock()
		defer d.sourceStreamsMutex.Unlock()
		d.masterContextCancelFunc()
		close(d.sinkStream)
		d.sourceStreams = d.sourceStreams[:0]
	})
}
