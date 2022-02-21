package knnc

import "time"

/*
This file contains a convenience type Pipeline which orchestrates some of the
concurrent KNN processes in this pkg.
*/

// Pipeline is a convenience type for connecting concurrent stages and
// feeding them ScanChan instances. This can be used with the SearchSpace(s)
// types of this pkg (both singular and plural) and the different pre-defined
// stage funcs, though that is optional.
type Pipeline struct {
	inputChan             chan ScanItem          // Faucet.
	inputChanClosedSignal *CancelSignal          // Signal for stopping faucet.
	outputChan            <-chan ScoreItems      // Sink.
	scanTick              ActiveGoroutinesTicker // Current n chans fed into faucet.
	cancel                *CancelSignal
	deadline              time.Duration
}

// NewPipelineArgs is intended as args for the NewPipeline func.
type NewPipelineArgs struct {
	// ScanChanBuffer specifies the ScanChan buffer in this pipeline.
	// This should be fairly large if multiple ScanChan instances are
	// added to this type through Pipeline.AddScanner(...).
	ScanChanBuffer int

	// Cancel is a way of explicitly cancelling the pipeline.
	Cancel *CancelSignal

	// BlockDeadline is a way of cancelling the pipeline, as an addition to
	// the Cancel field of this struct. When using Pipeline.AddScanner to add
	// ScanChan instances, a goroutine will forward that chan into the internal
	// pipeline. This forwarding is done with a goroutine and this variable
	// specifies how long it can block until cancelling.
	BlockDeadline time.Duration

	// MapStage is intended to be a concurrent stage where ScanChan is converted
	// to chan of ScoreItem, i.e ScanChan items (mathx.Distancer) are mapped to
	// ScoreItem instances (carrying distance scores in a knn ctx). Note that this
	// can be used with the MapStage func of this pkg (with closure conversion).
	MapStage func(ScanChan) (<-chan ScoreItem, bool)
	// FilterStage is intended to be a concurrent stage where a Scoreitem chan
	// is filteredi / converted to another ScoreItem chan. Note that this can
	// be used with the FilterStage func of this pkg (with closure conversion).
	FilterStage func(<-chan ScoreItem) (<-chan ScoreItem, bool)
	// General conceptual functionality of the pipeline is to collect ScoreItem
	// instances, then rank them into a slice to find KNN. As such, the MergeStage
	// func accepts a chan/stream of ScoreItem instances and returns a chan/stream
	// of ScoreItems (plural). Note that the MergeStage func of this pkg can be
	// used here (with closure conversion).
	MergeStage func(<-chan ScoreItem) (<-chan ScoreItems, bool)
}

// Ok validates NewPipelineArgs. Returns true iff:
//	(1) args.ScanChanBuffer >= 0,
//	(2)	args.MapStage != nil,
//	(3)	args.FilterStage != nil,
//	(4)	args.MergeStage != nil.
func (args *NewPipelineArgs) Ok() bool {
	return boolsOk([]bool{
		args.ScanChanBuffer >= 0,
		args.Cancel.c != nil,
		args.BlockDeadline > 0,
		args.MapStage != nil,
		args.FilterStage != nil,
		args.MergeStage != nil,
	})
}

// NewPipeline assembles the stage funcs in NewPipelineArgs into a pipeline.
// Will fail (return nil, false) if args.Ok() == false, or if any of the
// stage funcs return false when called. Note that no cleanup will be done
// if one stage succeeds and another fails, so all cleanup must be handled
// by the caller of this func.
func NewPipeline(args NewPipelineArgs) (*Pipeline, bool) {
	if !args.Ok() {
		return nil, false
	}

	chScan := make(chan ScanItem, args.ScanChanBuffer)
	chMap, ok := args.MapStage(chScan)
	if !ok || chScan == nil {
		return nil, false
	}
	chFilter, ok := args.FilterStage(chMap)
	if !ok || chFilter == nil {
		return nil, false
	}

	chFinal, ok := args.MergeStage(chFilter)
	if !ok {
		return nil, false
	}

	pipeline := Pipeline{
		inputChan:             chScan,
		inputChanClosedSignal: NewCancelSignal(),
		outputChan:            chFinal,
		cancel:                args.Cancel,
	}

	return &pipeline, true

}

// AddScanner connects a specified ScanChan to the internal ScanChan that is fed
// through the pipeline. Closing the internal chan is done with Pipeline.WaitThenClose.
// Will return false if a Pipeline.WaitThenClose is called previously or if s is nil.
func (p *Pipeline) AddScanner(s ScanChan) bool {
	if s == nil || p.inputChanClosedSignal.Cancelled() {
		return false
	}

	done := p.scanTick.AddAwait()
	go func() {
		defer done()
		for distancer := range s {
			select {
			case p.inputChan <- distancer:
			case <-p.cancel.c:
				return
			case <-time.After(p.deadline):
				return
			}
		}
	}()
	return true
}

// WaitThenClose sends a signal to stop accepting more data (with AddScanner
// method) to the internal 'faucet' channel that is fed through the pipeline,
// waits (blocking) for it to finish processing, then closes it. This cannot
// be undone and is intended to be called right after calling AddScanner for
// the last time.
func (p *Pipeline) WaitThenClose() bool {
	// In case this is called multiple times.
	if p.inputChanClosedSignal.Cancelled() {
		return false
	}
	p.inputChanClosedSignal.Cancel()
	p.scanTick.BlockUntilBelowN(1)
	close(p.inputChan)
	return true
}

// ConsumeIter lends access to the final step/stage in the pipeline, i.e acts as
// a sink. Specifically, it iterates over the final channel and passes the value
// to the receiver func. Returns false if receiver func is nil.
func (p *Pipeline) ConsumeIter(rcv func(ScoreItems) bool) bool {
	if rcv == nil {
		return false
	}
	for scoreItems := range p.outputChan {
		if p.cancel.Cancelled() {
			return true
		}
		if !rcv(scoreItems) {
			return true
		}
	}

	return true
}
