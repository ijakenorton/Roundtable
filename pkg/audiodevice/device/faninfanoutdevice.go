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
// Add and removing sinkStreams is concurrency safe thanks to a mutex.
type FanOutDevice struct {
	deviceProperties audiodevice.DeviceProperties
	// A master context to cancel ALL sinks at once
	// sink channel contexts will be spawned as sub-contexts of this one.
	masterContext               context.Context
	masterContextCancelFunction context.CancelFunc

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
		deviceProperties:            properties,
		masterContext:               masterContext,
		masterContextCancelFunction: masterContextCancelFunction,
		sinks:                       make([]fanOutSink, 0),
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
	d.masterContextCancelFunction()
	for _, sink := range d.sinks {
		sink.ctxCancel()
		close(sink.stream)
	}
	d.sinks = d.sinks[:0]
}

// --------------------------------------------------------------------------------
// Fan In Device (Many to One)
