package knnc

import (
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/mathx"
)

// Checks the core functionality of the Pipeline T, avoiding the usage of most
// other code in this pkg.
func TestPipelineMinimal(t *testing.T) {

	//dim := 3
	query := newTVec(0)
	pool := make([]mathx.Distancer, 0, 9)
	for i := 1; i < 9; i++ { // Note 1 - 9.
		pool = append(pool, newTVec(float64(i)))
	}

	k := 2            // k in knn
	ascending := true // Using Euclidean distance in this func

	// One faucet per vector. This is to check the Pipeline.AddScanner method.
	faucetChans := make([]ScanChan, 0, len(pool))
	for _, vec := range pool {
		out := make(chan ScanItem)
		go func(vec mathx.Distancer, out chan ScanItem) {
			defer close(out)
			out <- ScanItem{Distancer: vec}
		}(vec, out)

		faucetChans = append(faucetChans, out)
	}

	// Simply derive Euclidean distance between the query and all vecs in the pool
	mapStage := func(in ScanChan) (<-chan ScoreItem, bool) {
		out := make(chan ScoreItem)
		go func() {
			defer close(out)
			for scanItem := range in {
				score, ok := scanItem.Distancer.EuclideanDistance(query)
				if !ok {
					continue
				}
				out <- ScoreItem{Distancer: scanItem.Distancer, Score: score, Set: true}
			}
		}()

		return out, true
	}

	// Filter out scores worse than 3, so only 3 vecs in the pool pass this stage.
	filterStage := func(in <-chan ScoreItem) (<-chan ScoreItem, bool) {
		out := make(chan ScoreItem)
		go func() {
			defer close(out)
			for scoreItem := range in {
				if scoreItem.Score > 3 {
					continue
				}
				out <- scoreItem
			}
		}()

		return out, true
	}

	// Consume eagerly (for simplicity), so the returned chan will only yield 1 slice.
	mergeStage := func(in <-chan ScoreItem) (<-chan ScoreItems, bool) {
		out := make(chan ScoreItems)
		go func() {
			defer close(out)

			r := make(ScoreItems, k)
			for scoreItem := range in {
				r.BubbleInsert(scoreItem, ascending)
			}
			out <- r
		}()

		return out, true
	}

	pipeline, ok := NewPipeline(NewPipelineArgs{
		BaseWorkerArgs: BaseWorkerArgs{
			Buf:    1,
			Cancel: NewCancelSignal(),
			TTL:    time.Second * 10,
		},
		MapStage:    mapStage,
		FilterStage: filterStage,
		MergeStage:  mergeStage,
	})
	if !ok {
		t.Fatal("pipeline setup not ok")
	}

	go func() {
		for _, faucet := range faucetChans {
			if !pipeline.AddScanner(faucet) {
				panic("add after close")
			}
		}
		pipeline.WaitThenClose()
	}()

	// The consumeStage func consumes eagerly, result is put here.
	result := ScoreItems{}
	pipeline.ConsumeIter(func(scoreItems ScoreItems) bool {
		result = scoreItems
		return true
	})

	if len(result) != k {
		t.Fatal("Pipeline ended with unexpected result len:", len(result))
	}
	// Query vec={0}, best neigh={1}, best distance=1
	if result[0].Score != 1 {
		t.Fatal("Pipeline ended with an unexpected best neighbour:", result[0])
	}
}

// Using Pipeline T with SearchSpace, SearchSpaces, and all the stage-prefabs.
func TestPipelinePrefabbed(t *testing.T) {
	query := newTVec(0)
	cancel := NewCancelSignal()
	k := 2    // How many neighbours to get.
	n := 1000 // Amount of searchspaces (1 distancer each).

	// Used in all stages.
	uniformBaseStageArgs := BaseStageArgs{
		NWorkers: 10,
		BaseWorkerArgs: BaseWorkerArgs{
			Buf:    10,
			Cancel: cancel,
			TTL:    time.Second,
		},
	}

	ss := SearchSpaces{
		searchSpaces:            make([]*SearchSpace, 0, 9),
		searchSpacesMaxCap:      10,
		uniformVecDim:           3,
		maintenanceTaskInterval: time.Second,
		maintenanceActive:       false,
	}

	for i := 1; i < n; i++ { // Note, starts with 1.
		searchSpace := SearchSpace{
			items: []DistancerContainer{
				&data{v: newTVec(float64(i))},
			},
		}
		ss.searchSpaces = append(ss.searchSpaces, &searchSpace)
	}

	pipeline, ok := NewPipeline(NewPipelineArgs{
		BaseWorkerArgs: uniformBaseStageArgs.BaseWorkerArgs,
		MapStage: func(in ScanChan) (<-chan ScoreItem, bool) {
			return MapStage(MapStageArgs{
				In: in,
				MapStagePartialArgs: MapStagePartialArgs{
					MapFunc: func(other mathx.Distancer) (ScoreItem, bool) {
						score, ok := other.EuclideanDistance(query)
						return ScoreItem{Score: score}, ok
					},
					BaseStageArgs: uniformBaseStageArgs,
				},
			})

		},
		FilterStage: func(in <-chan ScoreItem) (<-chan ScoreItem, bool) {
			return FilterStage(FilterStageArgs{
				In: in,
				FilterStagePartialArgs: FilterStagePartialArgs{
					FilterFunc: func(scoreItem ScoreItem) bool {
						return scoreItem.Score > 0.9
					},
					BaseStageArgs: uniformBaseStageArgs,
				},
			})
		},
		MergeStage: func(in <-chan ScoreItem) (<-chan ScoreItems, bool) {
			return MergeStage(MergeStageArgs{
				In: in,
				MergeStagePartialArgs: MergeStagePartialArgs{
					K:             k,
					Ascending:     true,
					SendInterval:  1,
					BaseStageArgs: uniformBaseStageArgs,
				},
			})
		},
	})
	if !ok {
		t.Fatal("pipeline setup not ok")
	}

	scanChans, _ := ss.Scan(SearchSpacesScanArgs{
		Extent:        1,
		BaseStageArgs: uniformBaseStageArgs,
	})

	go func() {
		for scanChan := range scanChans {
			if !pipeline.AddScanner(scanChan) {
				t.Fatal("could not add scanChan to pipeline")
			}
		}

		pipeline.WaitThenClose()
	}()

	// The consumeStage func consumes eagerly, result is put here.
	result := make(ScoreItems, k)
	pipeline.ConsumeIter(func(scoreItems ScoreItems) bool {
		for _, scoreItem := range scoreItems {
			result.BubbleInsert(scoreItem, true)
		}
		return true
	})

	if len(result) != k {
		t.Fatal("Pipeline ended with unexpected result len:", len(result))
	}

	// Query vec={0}, best neigh={1}, best distance=1
	if result[0].Score != 1 {
		t.Fatal("Pipeline ended with an unexpected best neighbour:", result[0])
	}
}
