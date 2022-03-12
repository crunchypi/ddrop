package knnc

/*
This file contains a convenience type Pipeline which orchestrates some of the
concurrent KNN processes in this pkg.
*/

// Pipeline is a convenience type for connecting concurrent stages and
// feeding them ScanChan instances. This can be used with the SearchSpace(s)
// types of this pkg (both singular and plural) and the different pre-defined
// stage funcs, though that is optional.
type Pipeline struct {
	baseWorkerArgs BaseWorkerArgs

	inputChan             chan ScanItem          // Faucet.
	inputChanClosedSignal *CancelSignal          // Signal for stopping faucet.
	outputChan            <-chan ScoreItems      // Sink.
	scanTick              ActiveGoroutinesTicker // Current n chans fed into faucet.
}

// NewPipelineArgs is intended as args for the NewPipeline func.
type NewPipelineArgs struct {
	// Note that the Buf field here is used as buffer for the ScanChan
	// instances fed into the pipeline with Pipeline.AddScanner(...).
	// Also nota that the different stages here will not 'inherit' these
	// worker args.
	BaseWorkerArgs

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
//	(1) args.BaseWorkerArgs.Ok() == true,
//	(2)	args.MapStage != nil,
//	(3)	args.FilterStage != nil,
//	(4)	args.MergeStage != nil.
func (args *NewPipelineArgs) Ok() bool {
	return boolsOk([]bool{
		args.BaseWorkerArgs.Ok(),
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

	chScan := make(chan ScanItem, args.Buf)
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
		baseWorkerArgs:        args.BaseWorkerArgs,
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

	// Just for the deadline signal method
	deadlineSignal, deadlineSignalCancel := p.baseWorkerArgs.DeadlineSignal()
	defer deadlineSignalCancel.Cancel()

	go func() {
		defer done()
		for distancer := range s {
			select {
			case p.inputChan <- distancer:
			case <-p.baseWorkerArgs.Cancel.c:
				return
			case <-deadlineSignal.c:
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
		if p.baseWorkerArgs.Cancel.Cancelled() {
			return true
		}
		if !rcv(scoreItems) {
			return true
		}
	}

	return true
}
