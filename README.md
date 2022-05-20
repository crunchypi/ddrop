# ddrop

The project "ddrop" (abbreviation for dendrophobia) is a distributed KNN search engine, intended for benchmarking distributed "Approximate Nearest Neighbours Search" implementations such as LSH. 

Nearest neighbur: https://en.wikipedia.org/wiki/Nearest_neighbor_search
\
Approximate (ANN): https://en.wikipedia.org/wiki/Nearest_neighbor_search#Approximation_methods
\
Locality sensitive hashing (LSH): https://en.wikipedia.org/wiki/Locality-sensitive_hashing
\

As a brief overview, ddrop uses a concurrent KNN pipeline at the base with some performance improvements. There is a layer on top of that for handling KNN requests (queueing, monitoring, etc). On top of that is an RPC layer which orchestrates the previous two layers on a network-level. At the top is a JSON/REST server as a way of interfacing the system.

# Quickstart

This will cover the following:
- Compile + start the http server.
- Ping http server to check if it's ok.
- Configuration over http endpoints with JSON/POST.
- Ping rpc server/network to check cluster ok.
- Add data
- KNN request

**First** go to /cmd/simple-http-server and **build/run** the code. Running it as default will start the server at localhost:8080, with 10s read/write deadlines:
```bash
cd cmd/simple-http-server
go build -o ddrop .
./ddrop
```

Now to **ping**, one can send an empty json to the server (python code):
```python
import requests

resp = requests.post(
  url="http://localhost:8080/ping",
  json={}
)
# Should be status 200, with a returned bool True.
print(resp, resp.json())
```

Now to **configure**, one has to send a fairly lengthy json. This sets up the an internal rpc server for this node (along with all the knn stuff). Do note that there is some state involved with regards to the server (is forbidden to start a new rpc server without clearing the old one) and so this example is valid only for when the http server is freshly ran (more details further down). Anyway:
```python
import requests

# This refers to KNN search spaces, where the pool of vectors to check
# (the "neighbours" part of KNN) are kept and scanned.
new_search_spaces_args = {
  # Max amount of vectors a single search space can have.
  "searchSpacesMaxCap": 10000,
  # Max amount of search spaces. 
  "searchSpacesMaxN": 10000,
  # How often (in nanoseconds) will the maintenance cycle pause before checking
  # a vector for expiration. Note this is pause per vector, so should not be too
  "maintenanceTaskInterval": 10000000 # 10ms
}

# There is some performance tracking in the system. All of it is based on a linked
# list which tracks events over discrete amounts of time. This specifies the
# layout of the linked lists.
new_tracker_args = {
  # Length of the linked list.
  "maxChainLinkN": 60,
  # How much time will a single link cover.
  "minChainLinkSize": 1000000000, # 1 second.
  # When checking events in the linked list, how far back in time to check when
  # the span hasn't been specified? Rule of thumb, have this as "minChainLinkSize".
  "standardPeriod": 1000000000 # 1 second.
}

resp = requests.post(
  url="http://localhost:8080/ops/rpc/server/start",
  json={
    # Address for the new rpc server. Must not be the same as the http server.
    "rpcAddr": "localhost:8081",
    "cfg": {
      "newSearchSpacesArgs": new_search_spaces_args,
      # This is for tracking latency for only KNN queue and KNN queries.
      # This feature is always available.
      "newLatencyTrackerArgs": new_tracker_args,
      # This is for tracking more detailed stats for KNN queries, such
      # as average score and satiscation _in_ _addition_ to latency.
      # This feature is only available when KNN queries have allowed it,
      # as it is a little costly (more on that further down).
      "newKNNMonitorArgs": new_tracker_args,
      # Buffer of the KNN queue. Have this the same as the option below.
      "knnQueueBuf": 10,
      # Specifies how many KNN queries can be be done concurrently.
      # Note that this specifies the number of _parent_ green-threads per
      # query, and each query can use several green-threads by themselves.
      "knnQueueMaxConcurrent": 10,
    }
  }
)

# Expected after running on a new and clean http server:
# Status: 200
# Json: {'statusCode': 2, 'statusMsg': 'rpc server state: started'}
print(resp, resp.json())
```

Now to **ping** the rpc node. This will only work the block above is ran. Alternatively, it'll run if this http server is aware of any other rpc nodes on the network (not covered in the quickstart section). Anyway:
```python
import requests

resp = requests.post(
  url="http://localhost:8080/cmd/ping",
  json={}
)

# Status: 200
# Json can be something like this
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': True, # ping response.
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```

Now to **add data**. This sends it to the specified http server, which will distribute the data randomly on the rpc network. Additionally, there are a few rules for adding data, such as all vectors in a namespace must be of equal length/dimension, or not exceeding the search space capacity (as configured a bit further up). Anyway:
```python
import requests

resp = requests.post(
  url="http://localhost:8080/cmd/add",
  # Note, there are a couple additional options (such as expiration),
  # but they are not covered in this quickstart example.
  json=[
    {"namespace": "", "vec": [1,1,1]},
    {"namespace": "", "vec": [2,2,2]},
    {"namespace": "", "vec": [3,3,3]}
  ]
)

# Status: 200
# Json can be something like this
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': [True, True], # Bool status per vector.
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```

Now to finally do a *KNN* request. Note that some of the args are strict and will lead to query rejection or unexpected behavior if misconfigured. This is just an example, so see the detailed endpoint description further down. Anyway:
```python
import requests

resp = requests.post(
  url="http://localhost:8080/cmd/knn",
  json={
    # Multiple vecs can share the args. Note that this is the same
    # dimension as the added data above.
    "queryVecs": [ [0,0,0] ],
    # Note that some of these are strict, so 
    "args": {
      "namespace": "", # Same as in the example above.
      # How much resources to give this query; higher is more, can't be < 1.
      "priority": 1,
      # 0=Euclidean distance, 1=Cosine similarity.
      "KNNMethod": 0,
      # Are "best" scores lower?
      "ascending": True,
      # The K in KNN.
      "k":2,
      # How much of the search space / vec pool to search from 0 to 1.
      "extent": 1.0,
      # Accept and stop query after "k" items of "accept" scores.
      "accept": 1.0,
      # Reject all scores worse than this.
      "reject": 9.0,
      # Abort (with potentially no results) after this duration.
      "ttl": 1000000000, # 1 second.
      # Enable detailed monitoring for this query. This makes the query slower.
      "monitor": True,
    }
  }
)

# Status 200
# json:
# [ # One object per query vector.
#   {
#     # Vec that was used for querying the pool.
#     'queryVec': [0, 0, 0],
#     # Index of the query vec in the query.
#     'queryVecIndex': 0,
#     'results': [
#       {
#         # Addr of the rpc node who had this result.
#         'remoteAddr': 'localhost:8081',
#         # Network error.
#         'netErr': None,
#         # The vector that was found. Since this is the
#         # first vector in this 'results' list, then this
#         # is the best result (according to the cfg).
#         'payload': {'vec': [1, 1, 1],
#         # Distance score. We used Euclidean distance.
#         'score': 1.7320508075688772},
#         # http->rpc server latency in nanoseconds.
#         'networkLatency': 1505000
#       }, 
#       {
#         'remoteAddr': 'localhost:8081',
#         'netErr': None,
#         'payload': {'vec': [2, 2, 2],
#         'score': 3.4641016151377544},
#         'networkLatency': 1505000
#       }
#     ]
#   }
# ]
print(resp, resp.json())

```

