package knnc

import "testing"

func TestSearchSpaceAddSearchable(t *testing.T) {
	ss, ok := NewSearchSpace(2)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	if !ss.AddSearchable(&data{newTVecRand(3)}) {
		t.Fatal("could not add to fresh search space")
	}

	if ss.AddSearchable(&data{newTVecRand(9)}) {
		t.Fatal("vec dim consistency check failed")
	}

	if !ss.AddSearchable(&data{newTVecRand(3)}) {
		t.Fatal("could not reach search space cap")
	}

	ss.items = make([]DistancerContainer, 0, 10)
	if !ss.AddSearchable(&data{newTVecRand(9)}) {
		t.Fatal("vec dim consistency enforced even though ss is empty")
	}
}

func TestSearchSpaceClear(t *testing.T) {
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	ss.items = []DistancerContainer{
		&data{newTVecRand(1)},
		&data{newTVecRand(2)},
		&data{newTVecRand(3)},
	}

	ss.Clear()
	if ss.Len() != 0 {
		t.Errorf("unexpected len after clear: %v", ss.Len())
	}

}

func TestSearchSpaceClean(t *testing.T) {
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	e := 2.
	d := &data{newTVec(e)}

	ss.items = []DistancerContainer{
		&data{newTVec(e - 1)},
		d,
		&data{newTVec(e + 1)},
	}

	ss.Clean()
	if ss.Len() != 3 {
		t.Fatal("first clean removed non-nil item(s)")
	}

	d.v = nil // Mark for deletion.
	ss.Clean()
	if ss.Len() != 2 {
		t.Fatal("second clean did not remove the nil item")
	}

	e2, _ := ss.items[0].Distancer().Peek(0)
	e3, _ := ss.items[1].Distancer().Peek(0)
	if e2 == e || e3 == e {
		t.Error("incorrect item deleted")
	}
}
