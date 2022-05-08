package api

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/timex"
	"github.com/crunchypi/ddrop/service/ops"
	rman "github.com/crunchypi/ddrop/service/requestman"
)

// withNetIO is a convenience func for unpacking json requests (T) and packing
// json responses [U]. Note that T and U can be anything as long as it can be
// packed/unpacked as json. Also note that an empty struct for T signals that
// nothing is expected for input. It works with the following syntax:
//
//  // This will expect to recieve a string json and send back a bool resp.
//  withNetIO(w, r, func(opts string) bool {
//      fmt.Println("got:", opts)
//      // Simply return to pack- and send the response.
//      return true
//  })
//
// If T is not an empty struct (stuct{}) and cannot be decoded, then the
// rcv func will not run, this func will simply do w.WriteHeader with a
// http.StatusBadRequest, then return.
// Similarly, if U cannot be encoded, then this func will simply do a
// w.WriteHeader with http.StatusInternalServerError, then return.
func withNetIO[T, U any](
	w http.ResponseWriter,
	r *http.Request,
	rcv func(in T) (out U),
) {
	var in T

	// Only try to unpack request data if T is not empty struct.
	_, ok := any(in).(struct{})
	if !ok {
		// Read.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// T extract.
		if err := json.Unmarshal(body, &in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	out := rcv(in)

	// Try send back.
	b, err := json.Marshal(out)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

// newSearchSpacesArgs mirrors knnc.NewSearchSpacesArgs, see docs for that
// struct for more info. This is defined seperately for struct tags.
type newSearchSpacesArgs struct {
	SearchSpacesMaxCap      int           `json:"searchSpacesMaxCap"`
	SearchSpacesMaxN        int           `json:"searchSpacesMaxN"`
	MaintenanceTaskInterval time.Duration `json:"maintenanceTaskInterval"`
}

// export converts this instance into its exported equivalent in the knnc pkg.
func (args *newSearchSpacesArgs) export() knnc.NewSearchSpacesArgs {
	return knnc.NewSearchSpacesArgs{
		SearchSpacesMaxCap:      args.SearchSpacesMaxCap,
		SearchSpacesMaxN:        args.SearchSpacesMaxN,
		MaintenanceTaskInterval: args.MaintenanceTaskInterval,
	}
}

// newLatencyTrackerArgs mirrors timex.NewLatencyTrackerArgs, see docs for that
// struct for more info. This is defined seperately for struct tags.
type newLatencyTrackerArgs struct {
	MaxChainLinkN    int           `json:"maxChainLinkN"`
	MinChainLinkSize time.Duration `json:"minChainLinkSize"`
	StandardPeriod   time.Duration `json:"standardPeriod"`
}

// export converts this instance into its exported equivalent in the timex pkg.
func (args *newLatencyTrackerArgs) export() timex.NewLatencyTrackerArgs {
	return timex.NewLatencyTrackerArgs{
		MaxChainLinkN:    args.MaxChainLinkN,
		MinChainLinkSize: args.MinChainLinkSize,
		StandardPeriod:   args.StandardPeriod,
	}
}

// newRequestManagerHandleArgs mirrors (almost) requestmanager.NewHandleArgs,
// see docs for that struct for more info. This is redefined for struct tags.
// Note: The only difference is that the Ctx field is excluded (naturally).
type newRequestManagerHandleArgs struct {
	NewSearchSpacesArgs   newSearchSpacesArgs   `json:"newSearchSpacesArgs"`
	NewLatencyTrackerArgs newLatencyTrackerArgs `json:"newLatencyTrackerArgs"`
	KNNQueueBuf           int                   `json:"knnQueueBuf"`
	KNNQueueMaxConcurrent int                   `json:"knnQueueMaxConcurrent"`
	NewKNNMonitorArgs     newLatencyTrackerArgs `json:"newKNNMonitorArgs"`
}

// export converts this instance into its exported equivalent in the requestmanager pkg.
func (args *newRequestManagerHandleArgs) export(ctx context.Context) rman.NewHandleArgs {
	return rman.NewHandleArgs{
		NewSearchSpaceArgs:    args.NewSearchSpacesArgs.export(),
		NewLatencyTrackerArgs: args.NewLatencyTrackerArgs.export(),
		KNNQueueBuf:           args.KNNQueueBuf,
		KNNQueueMaxConcurrent: args.KNNQueueMaxConcurrent,
		Ctx:                   ctx,
		NewKNNMonitorArgs:     args.NewKNNMonitorArgs.export(),
	}
}

// rpcServerStartArgs is originally intended as json args/options for the
// "/ops/server/start" endpoint (method handle.RPCServerStart). It is used
// to start a new ops.Server with ops.NewServer.
type rpcServerStartArgs struct {
	Addr string                      `json:"rpcAddr"`
	Cfg  newRequestManagerHandleArgs `json:"cfg"`
}

// clientResult mirrors the _exported_ T of the same in pkg ops, see docs for
// that struct for more info. This is defined seperately for struct tags.
type clientResult[T any] struct {
	RemoteAddr     string        `json:"remoteAddr"`
	NetErr         error         `json:"netErr"`
	Payload        T             `json:"payload"`
	NetworkLatency time.Duration `json:"networkLatency"`
}

// newClientResult creates a clientResult from an ops.ClientResult. It uses
// "conv" to convert r.Payload into the payload of the new returned instance.
// This is useful because the new payload might have to be something that has
// struct tags.
func newClientResult[T, U any](
	r ops.ClientResult[T],
	conv func(T) U,
) clientResult[U] {
	return clientResult[U]{
		RemoteAddr:     r.RemoteAddr,
		NetErr:         r.NetErr,
		Payload:        conv(r.Payload),
		NetworkLatency: r.NetworkLatency,
	}
}

// newClientResults is similar to newClientResult but works with
// ops.ClientResults (plural).
func newClientResults[T, U any](
	r ops.ClientResults[T],
	conv func(T) U,
) []clientResult[U] {
	s := make([]clientResult[U], 0, 10)
	for chItem := range r {
		s = append(s, newClientResult(*chItem, conv))
	}
	return s
}

// addDataArgs mirrors the _exported_ T of the same in pkg ops, see docs for
// that struct for more info. This is defined seperately for struct tags.
type addDataArgs struct {
	Namespace string    `json:"namespace"`
	Vec       []float64 `json:"vec"`
	Data      []byte    `json:"data"`
	Expires   time.Time `json:"expired"`
}

// export converts this instance into its exported equivalent in the ops pkg.
func (args *addDataArgs) export() ops.AddDataArgs {
	return ops.AddDataArgs{
		Namespace: args.Namespace,
		Vec:       args.Vec,
		Data:      args.Data,
		Expires:   args.Expires,
	}
}

// knnArgsPartial is exactly the same as requestmanager.KNNArgs except for the
// missing QueryVec field. It is re-defined here for two reasons:
// 1) Struct tags for json.
// 2) Decoupling the QueryVec field allows for using these KNN args with multiple
//    different query vecs, making API calls more efficient.
type knnArgsPartial struct {
	Namespace string         `json:"namespace"`
	Priority  int            `json:"priority"`
	KNNMethod rman.KNNMethod `json:"KNNMethod"`
	Ascending bool           `json:"ascending"`
	K         int            `json:"k"`
	Extent    float64        `json:"extent"`
	Accept    float64        `json:"accept"`
	Reject    float64        `json:"reject"`
	TTL       time.Duration  `json:"ttl"`
	Monitor   bool           `json:"monitor"`
}

// knnArgs is intended as json args/options for the "/cmd/knn" endpoint (method
// handle.RPCKNNEager). It is used for making multiple knn queries with partial
// knnArgs.
type knnArgs struct {
	QueryVecs [][]float64    `json:"queryVecs"`
	Args      knnArgsPartial `json:"args"`
}

// export converts this instance into multiple requestmanager.KNNArgs. The fmt
// is: one KNNArgs per knnArgs.QueryVecs.
func (args *knnArgs) export() []rman.KNNArgs {
	r := make([]rman.KNNArgs, len(args.QueryVecs))
	for i, vec := range args.QueryVecs {
		r[i] = rman.KNNArgs{
			Namespace: args.Args.Namespace,
			Priority:  args.Args.Priority,
			QueryVec:  vec,
			KNNMethod: args.Args.KNNMethod,
			Ascending: args.Args.Ascending,
			K:         args.Args.K,
			Extent:    args.Args.Extent,
			Accept:    args.Args.Accept,
			Reject:    args.Args.Reject,
			TTL:       args.Args.TTL,
			Monitor:   args.Args.Monitor,
		}
	}
	return r
}

// knnRespItem mirrors the ops.KNNRespItem. It is re-defined for struct tags.
type knnRespItem struct {
	Vec   []float64 `json:"vec"`
	Score float64   `json:"score"`
}

// knnResp is similar to ops.KNNResp but modified/expanden for the purposes
// of this pkg. Specifically, it also contains query vec (from QueryVecs field
// of T knnArgs _and_ its index for client convenience.
type knnResp struct {
	QueryVec      []float64                   `json:"queryVec"`
	QueryVecIndex int                         `json:"queryVecIndex"`
	Results       []clientResult[knnRespItem] `json:"results"`
}

// sSpaceDimResp mirrors the _exported_ T of the same in pkg ops, see docs for
// that struct for more info. This is defined seperately for struct tags.
type sSpaceDimResp struct {
	LookupOk bool `json:"lookupOk"`
	Dim      int  `json:"dim"`
}

// sSpaceLenResp mirrors the _exported_ T of the same in pkg ops, see docs for
// that struct for more info. This is defined seperately for struct tags.
type sSpaceLenResp struct {
	LookupOk bool `json:"lookupOk"`
	NSSpaces int  `json:"nSearchSpaces"`
	NVecs    int  `json:"nVecs"`
}

// sSpaceCapResp mirrors the _exported_ T of the same in pkg ops, see docs for
// that struct for more info. This is defined seperately for struct tags.
type sSpaceCapResp struct {
	LookupOk bool `json:"lookupOk"`
	Cap      int  `json:"cap"`
}

// knnLatencyArgs mirrors ops.KNNLatencyArgs; see docs for that struct for more
// info. This is redefined seperately for struct tags.
type knnLatencyArgs struct {
	Key    string        `json:"key"`
	Period time.Duration `json:"period"`
}

// knnLatencyResp mirrors ops.KNNLatencyResp; see docs for that struct for more
// info. This is redefined seperately for struct tags.
type knnLatencyResp struct {
	LookupOk bool          `json:"lookupOk"`
	Queue    time.Duration `json:"queue"`
	Query    time.Duration `json:"query"`
	BoundsOk bool          `json:"boundsOk"`
}

// knnMonArgs mirrors ops.KNNMonArgs; see docs for that struct for more info.
// This is redefined seperately for struct tags.
type knnMonArgs struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// knnMonItemAvg mirrors _almost requestman.KNNMonItemAvg; see docs for that
// struct for more info. This is redefined seperately for struct tags.
// Note, the only difference is that this struct excludes private fields.
type knnMonItemAvg struct {
	Created         time.Time     `json:"created"`
	Span            time.Duration `json:"span"`
	N               int           `json:"n"`
	NFailed         int           `json:"nFailed"`
	AvgLatency      time.Duration `json:"avgLatency"`
	AvgScore        float64       `json:"avgScore"`
	AvgScoreNoFails float64       `json:"avgScoreNoFails"`
	AvgSatisfaction float64       `json:"avgSatisfaction"`
}
