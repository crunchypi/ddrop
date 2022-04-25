package ops

import (
	"context"
	"math/rand"
	"sync"
	"time"

	rman "github.com/crunchypi/ddrop/service/requestman"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Clients T is used for composite Client calls.
type Clients struct {
	RemoteAddrs []string
	Timeout     time.Duration // This is passed to each individual Client.
}

// NewClients sets up a new composite client. If a timeout isn't specified, or has
// time.Duration <= 0, then the timeout will be set to 3 seconds as default.
func NewClients(remoteAddrs []string, timeout ...time.Duration) *Clients {
	if len(timeout) == 0 || timeout[0] <= time.Duration(0) {
		return &Clients{RemoteAddrs: remoteAddrs, Timeout: time.Second * 3}
	}
	return &Clients{RemoteAddrs: remoteAddrs, Timeout: timeout[0]}
}

// ClientResults is the general return from T Clients methods.
type ClientResults[T any] <-chan *ClientResult[T]

// collectToMap converts the chan into a map where keys are addresses, while
// values are the individual associated ClientResult.
func (cr *ClientResults[T]) collectToMap() map[string]*ClientResult[T] {
	m := make(map[string]*ClientResult[T])
	for item := range *cr {
		m[item.RemoteAddr] = item
	}

	return m
}

// fanInRequestsArgs is intended as args for fanInRequests(...).
type fanInRequestsArgs[T any] struct {
	// addrs is each address to do requests on. These will be used as the
	// first arg for NewClient(...), of which the return (*Client) is passed
	// to the requestFunc further down in this struct.
	addrs []string
	// ttl specifies the timeout for each Client call. This will be used as the
	// second arg for NewClient(...), of which the return (*Client) is passed
	// to the requestFunc further down in this struct.
	ttl time.Duration
	// requestFunc lends a *Client, which must be used to do requests.
	requestFunc func(c *Client) *ClientResult[T]
}

// fanInRequests is a shorthand for fan-out-requests-fan-in-responses.
// It is used to do multiple Client->Server calls concurrently. See
// docs for fanInRequestArgs for more details.
func fanInRequests[T any](args fanInRequestsArgs[T]) ClientResults[T] {
	ch := make(chan *ClientResult[T], len(args.addrs))
	wg := sync.WaitGroup{}
	wg.Add(len(args.addrs))

	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(args.ttl),
	)

	go func() {
		for _, addr := range args.addrs {
			go func(addr string) {
				defer wg.Done()
				select {
				case <-ctx.Done():
				case ch <- args.requestFunc(NewClient(addr, args.ttl)):
				}
			}(addr)
		}

		wg.Wait()
		close(ch)
		cancel()
	}()

	return ch
}

// Ping does a composite call to Client.Ping(), using all internal addrs.
// See docs for that method for more details.
func (cs *Clients) Ping() ClientResults[bool] {
	// Nested return type.
	type T = bool

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Ping()
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       cs.RemoteAddrs,
		ttl:         cs.Timeout,
		requestFunc: rf,
	})
}

// AddData does a composite call to Client.AddData(), using all internal addrs.
// Do note that the data to add (i.e "args") is added to a single remote node,
// picked at random, as a way of avoiding data duplication.
// See docs for that method for more details.
func (cs *Clients) AddData(args []AddDataArgs) ClientResults[[]bool] {
	// Nested return type.
	type T = []bool

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.AddData(args)
	}

	// Random addr.
	rIndex := rand.Intn(len(cs.RemoteAddrs))
	rAddr := cs.RemoteAddrs[rIndex]

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       []string{rAddr},
		ttl:         cs.Timeout,
		requestFunc: rf,
	})
}

// KNNEager does a composite call to Client.KNNEager(), using all internal addrs.
// See docs for that method for more details. Also see Clients.KNNEagerx for
// merging and ordering the results.
func (cs *Clients) KNNEager(args rman.KNNArgs) ClientResults[KNNResp] {
	// Nested return type.
	type T = KNNResp

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.KNNEager(args)
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       cs.RemoteAddrs,
		ttl:         cs.Timeout,
		requestFunc: rf,
	})

}

// KNNEagerx is a convenience on top of Clients.KNNEager. It calls the latter
// method, then orders KNN results into max args.K. The return is a flat slice
// of ClientResult containing a single KNNRespItem, where lower indexes are
// better KNN. It can look something like the following (simplified, using
// cosine similarity where higher scores are better):
// [
//   addr: ":3000", score: 0.99, Vec: ...,
//   addr: ":3000", score: 0.98, Vec: ...,
//   addr: ":3001", score: 0.97, Vec: ...,
// ]
// This is to include network information in addition to actual KNN results.
func (cs *Clients) KNNEagerx(args rman.KNNArgs) []*ClientResult[KNNRespItem] {
	// Used as the 'data' field in a sortItem.
	type U struct {
		clientResult *ClientResult[KNNResp]
		knnRespItem  KNNRespItem
	}

	sortItems := make([]sortItem[U], args.K)
	// Requests -> bubble insert client results into the sortItems var above.
	for clientResult := range cs.KNNEager(args) {
		// Validate / check skip.
		ok := true
		ok = ok && clientResult.NetErr == nil
		ok = ok && clientResult.Payload.Ok
		ok = ok && clientResult.Payload.KNN != nil
		if !ok {
			continue
		}

		// Insert.
		for _, knnItem := range clientResult.Payload.KNN {
			newSortItem := sortItem[U]{
				score: knnItem.Score,
				set:   true,
				data: U{
					clientResult: clientResult,
					knnRespItem:  knnItem,
				},
			}
			bubbleInsert(sortItems, newSortItem, args.Ascending)
		}
	}

	// Extract from ordered slice.
	r := make([]*ClientResult[KNNRespItem], 0, args.K)
	for _, sortItem := range sortItems {
		if !sortItem.set {
			continue
		}
		newClientResult := ClientResult[KNNRespItem]{
			RemoteAddr:     sortItem.data.clientResult.RemoteAddr,
			NetErr:         nil,
			Payload:        sortItem.data.knnRespItem,
			NetworkLatency: sortItem.data.clientResult.NetworkLatency,
		}
		r = append(r, &newClientResult)
	}

	return r
}
