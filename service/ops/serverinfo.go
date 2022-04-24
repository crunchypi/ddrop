package ops

import (
	"time"

	rman "github.com/crunchypi/ddrop/service/requestman"
)

// Info namespace of server (same as CInfo for Client). Registered with rpc in
// the Server.StartListen method.
type SInfo Server

// SSpaceNamespaces forwards the call to the method with the same name on top of
// the internal requestman.Handle.Info(). See docs for that for more details.
func (i *SInfo) SSpaceNamespaces(args SArgs[bool], resp *SResp[[]string]) error {
	resp.RecvTime = time.Now()

	resp.Payload = i.rManHandle.Info().SSpaceNamespaces()
	return nil
}

// SSpaceNamespace forwards the call to the method with the same name on top of
// the internal requestman.Handle.Info(). See docs for that for more details.
func (i *SInfo) SSpaceNamespace(args SArgs[string], resp *SResp[bool]) error {
	resp.RecvTime = time.Now()

	resp.Payload = i.rManHandle.Info().SSpaceNamespace(args.Payload)
	return nil
}

// SSpaceDim forwards the call to the method with the same name on top of
// the internal requestman.Handle.Info(). See docs for that for more details.
func (i *SInfo) SSpaceDim(args SArgs[string], resp *SResp[SSpaceDimResp]) error {
	resp.RecvTime = time.Now()

	dim, nsOk := i.rManHandle.Info().SSpaceDim(args.Payload)
	resp.Payload.LookupOk = nsOk
	resp.Payload.Dim = dim
	return nil
}

// SSpaceLen forwards the call to the method with the same name on top of
// the internal requestman.Handle.Info(). See docs for that for more details.
func (i *SInfo) SSpaceLen(args SArgs[string], resp *SResp[SSpaceLenResp]) error {
	resp.RecvTime = time.Now()

	nSSpaces, nVecs, nsOk := i.rManHandle.Info().SSpaceLen(args.Payload)
	resp.Payload.LookupOk = nsOk
	resp.Payload.NSSpaces = nSSpaces
	resp.Payload.NVecs = nVecs
	return nil
}

// SSpaceCap forwards the call to the method with the same name on top of
// the internal requestman.Handle.Info(). See docs for that for more details.
func (i *SInfo) SSpaceCap(args SArgs[string], resp *SResp[SSpaceCapResp]) error {
	resp.RecvTime = time.Now()

	capacity, nsOk := i.rManHandle.Info().SSpaceCap(args.Payload)
	resp.Payload.LookupOk = nsOk
	resp.Payload.Cap = capacity
	return nil
}

// KNNLatency forwards the call to the following methods of the internal
// requestman.Handle:
// - requestman.Handle.Info().KNNQueueLatency(...)
// - requestman.Handle.Info().KNNQueryLatency(...)
// See docs for those methods for more details about args, returns, etc.
func (i *SInfo) KNNLatency(args SArgs[KNNLatencyArgs], resp *SResp[KNNLatencyResp]) error {
	resp.RecvTime = time.Now()

	// requestman.Handle.Info().SearchSpaceLatency returns false if there is either
	// a lookup-no-ok _or_ if the passed duration is out of bounds. The resp here,
	// however, requires a clear indicator for whether the ns exists.
	nsOk := i.rManHandle.Info().SSpaceNamespace(args.Payload.Key)

	d := args.Payload.Period
	lQueue, ok1 := i.rManHandle.Info().KNNQueueLatency(d)
	lQuery, ok2 := i.rManHandle.Info().KNNQueryLatency(args.Payload.Key, d)

	resp.Payload.LookupOk = nsOk
	resp.Payload.Queue = lQueue
	resp.Payload.Query = lQuery
	resp.Payload.BoundsOk = ok1 && (ok2 && nsOk)
	return nil
}

// KNNMonitor forwards the call to the method with the same name on top of
// the internal requestman.Handle.Info(). See docs for that for more details.
func (i *SInfo) KNNMonitor(args SArgs[KNNMonArgs], resp *SResp[rman.KNNMonItemAvg]) error {
	resp.RecvTime = time.Now()
	resp.Payload = i.rManHandle.Info().KNNMonitor(
		args.Payload.Start,
		args.Payload.End,
	)

	return nil
}
