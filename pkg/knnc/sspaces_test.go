package knnc

import (
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestSearchSpacesAddSearchable(t *testing.T) {
	args := NewSearchSpacesArgs{
		SearchSpacesMaxCap:      10,
		SearchSpacesMaxN:        10,
		MaintenanceTaskInterval: time.Second,
	}
	ss, _ := NewSearchSpaces(args)

	if ss.AddSearchable(nil) {
		t.Fatal("SearchSpaces instance added nil value")
	}

	if ss.AddSearchable(&data{}) {
		t.Fatal("SearchSpaces instance added a DistancerContainer with a nil Distancer")
	}

	if !ss.AddSearchable(&data{v: newTVecRand(3)}) {
		t.Fatal("SearchSpaces instance did not add a valid DistancerContainer")
	}

	if ss.AddSearchable(&data{v: newTVecRand(4)}) {
		s := "SearchSpaces instance added a DistancerContainer with a vec "
		s += "dim that does not match the vec dim of existing additions"
		t.Fatal(s)
	}

	ss.searchSpaces = make([]*SearchSpace, 0, args.SearchSpacesMaxCap)
	if !ss.AddSearchable(&data{v: newTVecRand(4)}) {
		s := "SearchSpaces instance did not add a DistancerContainer with a vec "
		s += "dim that does not match the vec dim of previous additions which "
		s += "have been cleared"
		t.Fatal(s)
	}

	ss, _ = NewSearchSpaces(NewSearchSpacesArgs{
		SearchSpacesMaxCap:      1,
		SearchSpacesMaxN:        1,
		MaintenanceTaskInterval: time.Second,
	})

	if !ss.AddSearchable(&data{v: newTVecRand(3)}) {
		t.Fatal("SearchSpaces could not add a Distancer when MaxSearchSpaceN = 1")
	}
	if ss.AddSearchable(&data{v: newTVecRand(3)}) {
		t.Fatal("SearchSpaces added a second Distancer when MaxSearchSpaceN = 1")
	}
}

func TestSearchSpacesClean(t *testing.T) {
	ttl := time.Millisecond * 10
	ss, _ := NewSearchSpaces(NewSearchSpacesArgs{
		SearchSpacesMaxCap:      10,
		SearchSpacesMaxN:        10,
		MaintenanceTaskInterval: time.Second,
	})

	dataSlice := []*data{
		{v: newTVec(1), Expires: time.Now().Add(ttl)},
		{v: newTVec(2)},
		{v: newTVec(3), Expires: time.Now().Add(ttl)},
	}

	for _, d := range dataSlice {
		searchSpace := SearchSpace{items: []DistancerContainer{d}}
		ss.searchSpaces = append(ss.searchSpaces, &searchSpace)
	}

	ss.Clean()
	if len(ss.searchSpaces) != 3 {
		t.Fatalf("unexpected delete")
	}

	time.Sleep(ttl)
	ss.Clean()

	if len(ss.searchSpaces) != 1 {
		l := len(ss.searchSpaces) // Shorter line length.
		t.Fatal("did not clean enough data. remainder:", l)
	}
}

// Test verifies that output of SearchSpaces.Scan is ok in SearchSpaces.Scan.
// Does not cover the controlled-scan behaviour (goroutine suppression)
// NOTE: the correctness here is dependant on SearchSpace T.
func TestSearchSpacesScanOutputCorrectness(t *testing.T) {
	vecs := []*tVec{newTVec(1), newTVec(2), newTVec(3)}

	ss := SearchSpaces{
		searchSpaces: []*SearchSpace{
			{items: []DistancerContainer{&data{v: vecs[0]}}},
			{items: []DistancerContainer{&data{v: vecs[1]}}},
			{items: []DistancerContainer{&data{v: vecs[2]}}},
		},
		searchSpacesMaxCap:      10,
		uniformVecDim:           3,
		maintenanceTaskInterval: 1,     // Does not matter.
		maintenanceActive:       false, // Does not matter.
	}

	scanArgs := SearchSpacesScanArgs{
		Extent: 1.,
		BaseStageArgs: BaseStageArgs{
			NWorkers: 10, // Should not matter.
			BaseWorkerArgs: BaseWorkerArgs{
				Buf:           10,
				Cancel:        NewCancelSignal(),
				BlockDeadline: time.Second,
			},
		},
	}

	scanChans, _ := ss.Scan(scanArgs)
	results := make(map[float64]bool)
	for scanChan := range scanChans {
		for scanItem := range scanChan {
			element, _ := scanItem.Distancer.Peek(0)

			_, exists := results[element]
			if exists {
				t.Fatal("extracted duplicate from SearchSpaces.Scan:", element)
			}
			results[element] = true
		}
	}

	//  Check that all 'vectors' are accounted for.
	for _, vec := range vecs {
		elm, _ := vec.Peek(0)
		if _, exists := results[elm]; !exists {
			t.Fatal("SearchSpaces.Scan did not give all elements. missing:", vec)
		}
	}
}

// Test verifies the controlled-scan behaviour (goroutine suppression) in SearchSpaces.Scan.
// Does not cover the output correctness itself.
func TestSearchSpacesScanInternalBehaviourCorrectness(t *testing.T) {
	startGoroutineN := runtime.NumGoroutine()

	nWorkers := 10
	nSearchSpaces := nWorkers * 10
	nDistancers := nSearchSpaces * 10

	// account for the goroutine running the Scan method itself and the
	// goroutine in it that is used by time.After func in the select block.
	overhead := 2
	expectedMax := startGoroutineN + overhead + nWorkers

	// Used to check if the scanner actually goes up to expectedMax.
	lowestN := 0

	ss := SearchSpaces{
		searchSpaces:            make([]*SearchSpace, 0, nSearchSpaces),
		searchSpacesMaxCap:      10,    // Does not matter.
		uniformVecDim:           3,     // Does not matter.
		maintenanceTaskInterval: 1,     // Does not matter.
		maintenanceActive:       false, // Does not matter.
	}

	// Fill with random data.
	for i := 0; i < nSearchSpaces; i++ {
		distancers := make([]DistancerContainer, 0, nDistancers)
		for j := 0; j < nDistancers; j++ {
			distancers = append(distancers, &data{v: newTVecRand(3)})
		}

		newSearchSpace := SearchSpace{items: distancers}
		ss.searchSpaces = append(ss.searchSpaces, &newSearchSpace)

	}

	scanArgs := SearchSpacesScanArgs{
		Extent: 1.,
		BaseStageArgs: BaseStageArgs{
			NWorkers: nWorkers,
			BaseWorkerArgs: BaseWorkerArgs{
				Buf:           10,
				Cancel:        NewCancelSignal(),
				BlockDeadline: time.Second,
			},
		},
	}

	scanChans, _ := ss.Scan(scanArgs)
	for scanChan := range scanChans {
		// This is the point of the test; verify that the number of concurrent
		// goroutines does not exceed nWorkers. Not necessary to do it in the
		// scoreItem loop below.

		runtime.GC() // The GC can lag, so force it now.

		current := runtime.NumGoroutine()
		if current > lowestN {
			lowestN = current
		}

		if current > expectedMax {
			s := "SearchSpaces.Scan exceeded expected goroutines by %v. total=%v"
			t.Fatalf(s, current-expectedMax, current)
		}

		// Just drain without using val.
		for range scanChan {
		}
	}

	if lowestN != expectedMax {
		t.Fatalf("expected num goroutines was not reached. had: %v, wanted:%v",
			lowestN, expectedMax)
	}

	// TODO: The block below has unexpected behavior, i.e it works on one machine
	// but not on another. The block simply checks if current number of goroutines
	// is the same as at the start of the test. The start of the test _should_ be
	// 2, and that should be the same as calling runtime.NumGoroutine() now. But
	// for some reason, the start of the test has 4. Might be a go config thing
	// but i'm leaving this unresolved for now.

	// Give time for goroutines to end and gc to do it's thing.
	/*
		runtime.GC()
		time.Sleep(time.Millisecond * 200)
		if startGoroutineN != runtime.NumGoroutine() {
			t.Fatal("test start & end have neq amount of active goroutines", runtime.NumGoroutine())
		}
	*/
}

// Test covers the cleaning functionality of SearchSpaces.StartMaintenance.
// State management (e.g SearchSpaces.maintaining) is not checked here.
func TestSearchSpacesMaintenanceCleaning(t *testing.T) {

	interval := time.Duration(time.Millisecond * 100)

	ss, _ := NewSearchSpaces(NewSearchSpacesArgs{
		SearchSpacesMaxCap:      10,
		SearchSpacesMaxN:        10,
		MaintenanceTaskInterval: interval,
	})

	dataSlice := []*data{
		{v: newTVecRand(3)},
		{v: newTVec(9, 9, 9)},
		{v: newTVecRand(3)},
	}

	for _, d := range dataSlice {
		searchSpace := SearchSpace{items: []DistancerContainer{d}}
		ss.searchSpaces = append(ss.searchSpaces, &searchSpace)
	}

	// Separate process where dataSlice[0] and dataSlice[2] are disabled.
	markDeleteIndexes := []int{0, 2}
	wg := sync.WaitGroup{}
	wg.Add(len(markDeleteIndexes))
	go func() {
		for _, index := range markDeleteIndexes {
			time.Sleep(interval)
			// No locking since SearchSpaces have a read-only relationship with 'distancers'.
			dataSlice[index].v = nil
			wg.Done()
		}
	}()

	ss.StartMaintenance()
	// Give some time for deletions.
	time.Sleep(interval * time.Duration(len(dataSlice)*2))
	wg.Wait()

	if len(ss.searchSpaces) != 1 {
		s := "SearchSpaces auto-maintenance didn't clean all stale items"
		t.Fatal(s, len(ss.searchSpaces))
	}

	if reflect.ValueOf(ss.searchSpaces[0].items[0].Distancer()).IsNil() {
		s := "Remaining DistancerContainer in SearchSpaces.searchSpaces gives nil distancer"
		t.Fatal(s)
	}

	if elm, _ := ss.searchSpaces[0].items[0].Distancer().Peek(0); elm != 9 {
		t.Fatal("Incorrect distancer cleaning, want 9 as first elm, got:", elm)
	}
}

// Test covers state management related to SearchSpaces.StartMaintenance.
// Cleaning functionality itself is not checked here.
func TestSearchSpacesMaintenanceStates(t *testing.T) {
	startGoroutineN := runtime.NumGoroutine()

	interval := time.Duration(time.Millisecond * 100)

	ss := SearchSpaces{
		searchSpaces: []*SearchSpace{
			{items: []DistancerContainer{&data{}}},
			{items: []DistancerContainer{&data{v: newTVecRand(3)}}},
			{items: []DistancerContainer{&data{}}},
		},
		searchSpacesMaxCap:      10, // Does not matter.
		uniformVecDim:           3,  // Does not matter.
		maintenanceTaskInterval: interval,
		maintenanceActive:       false,
	}

	ss.StartMaintenance()
	if !ss.CheckMaintenance() {
		t.Fatal("SearchSpace started maintenance but does not signal that")
	}

	ss.StopMaintenance()
	// Give the maintenance procedure time to finish.
	time.Sleep(interval * time.Duration(2))

	// Give time for goroutines to end and gc to do it's thing.
	runtime.GC()
	time.Sleep(time.Millisecond * 200)
	if startGoroutineN != runtime.NumGoroutine() {
		t.Fatal("test start & end have neq amount of active goroutines")
	}
}
