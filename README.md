# ddrop

The project "ddrop" (abbreviation for dendrophobia) is a distributed KNN search engine, intended for benchmarking distributed "Approximate Nearest Neighbours Search" implementations such as LSH. 

Nearest neighbur: https://en.wikipedia.org/wiki/Nearest_neighbor_search
\
Approximate (ANN): https://en.wikipedia.org/wiki/Nearest_neighbor_search#Approximation_methods
\
Locality sensitive hashing (LSH): https://en.wikipedia.org/wiki/Locality-sensitive_hashing
\

As a brief overview, ddrop uses a concurrent KNN pipeline at the base with some performance improvements. There is a layer on top of that for handling KNN requests (queueing, monitoring, etc). On top of that is an RPC layer which orchestrates the previous two layers on a network-level. At the top is a JSON/REST server as a way of interfacing the system.

Sections:
- [Quickstart](#quickstart)
- [Endpoints](#endpoints)





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

# Namespace for where to put data on each rpc node. A new namespace will
# be created if it does not already exist.
ns = ""

resp = requests.post(
  url="http://localhost:8080/cmd/add",
  # Note, there are a couple additional options (such as expiration),
  # but they are not covered in this quickstart example.
  json=[
    {"namespace": ns, "vec": [1,1,1]},
    {"namespace": ns, "vec": [2,2,2]},
    {"namespace": ns, "vec": [3,3,3]}
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





# Endpoints

First, an overview.


- [http://ip:addr/ping](#ep00)

These are for managing the internal rpc server and rpc discovery.
- [http://ip:addr/ops/rpc/addrs/put](#ep01)
- [http://ip:addr/ops/rpc/addrs/get](#ep02)
- [http://ip:addr/ops/rpc/server/stop](#ep03)
- [http://ip:addr/ops/rpc/server/start](#ep04)

Orchestration of basic rpc actions.
- [http://ip:addr/cmd/ping](#ep05)
- [http://ip:addr/cmd/add](#ep06)
- [http://ip:addr/cmd/knn](#ep07)

Orchestration of rpc actions related to info/metadata features.
- [http://ip:addr/info/namespaces](#ep08)
- [http://ip:addr/info/namespace](#ep09)
- [http://ip:addr/info/dim](#ep10)
- [http://ip:addr/info/len](#ep11)
- [http://ip:addr/info/cap](#ep12)
- [http://ip:addr/info/knnLatency](#ep13)
- [http://ip:addr/info/knnMonitor](#ep14)



--- 
<div id=ep00><b>http://ip:addr/ping</b></div>
This is for simply pinging the http server

```python
import requests

resp = requests.post(
  url="http://localhost:8080/ping",
  json={}
)
# Should be status 200, with a returned bool True.
print(resp, resp.json())
```

---
<div id=ep01><b>http://ip:addr/ops/rpc/addrs/put</b></div>

The http server is used to contact and orchestrate the rpc network, but will need to know the relevant addresses -- this is the endpoint for doing the registry. Here are a couple notes:
- Addresses added here must not be the same as http server addresses.
- Addresses are kept as a set, so double registry is ok
- Addresses are refreshed over time, so stale ones that can't be contacted with [http://ip:addr/cmd/ping](#ep05) will be auto-deleted

```python
import requests

resp = requests.post(
  url="http://localhost:8080/ops/rpc/addrs/put",
  json=["192.168.0.4:8081"]
)
# Status: 200
# Json: list of known addresses for this http server (port 8080)
print(resp, resp.json())
```

---
<div id=ep02><b>http://ip:addr/ops/rpc/addrs/get</b></div>

This retrieves all the rpc network addresses that this http server knows. It is similar to [http://ip:addr/ops/rpc/addrs/put](#ep01) in that the response gives a list of addresses, but differs by not having to add a new one.

```python
import requests

resp = requests.post(
  url="http://localhost:8080/ops/rpc/addrs/get",
  json={}
)
# Status: 200
# Json: list of known addresses for this http server (port 8080)
print(resp, resp.json())
```  

---
<div id=ep03><b>http://ip:addr/ops/rpc/server/stop</b></div>

A new rpc server (and all the knn stuff) is started with [http://ip:addr/ops/rpc/server/start](#ep04) and this endpoint stops it. There are a few states for a starting/stopping an rpc server: starting, started, stopping, stopped.

```python
import requests

resp = requests.post(
  url="http://localhost:8080/ops/rpc/server/stop",
  json={}
)
# Status: 200
# JSON if an rpc server existed and was stopped _or_ if none existed. 
#   {'statusCode': 4, 'statusMsg': 'rpc server state: stopped'}
# JSON if this call happens twice concurrently (clash):
#   {'statusCode': 3, 'statusMsg': 'rpc server state: stopping'}
print(resp, resp.json())
```  

---
<div id=ep04><b>http://ip:addr/ops/rpc/server/start</b></div>

This is for starting an rpc server (and all knn stuff) within the http server (stopped with [http://ip:addr/ops/rpc/server/stop](#ep03)). Note that this automatically registers the given rpc address within this http server (has to be registered on all other nodes if using a cluster).

```python
import requests

# This refers to KNN search spaces, where the pool of vectors to check
# (the "neighbours" part of KNN) are kept and scanned. There is an explicit
# limitation of number of vectors as a way of keeping things in memory.
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
# JSON if an rpc server was successfully started: 
#   {'statusCode': 2, 'statusMsg': 'rpc server state: started'}
# JSON if this call happens twice concurrently (clash and maybe fail).
#   {'statusCode': 1, 'statusMsg': 'rpc server state: starting'}
print(resp, resp.json())
```  

---
<div id=ep05><b>http://ip:addr/cmd/ping</b></div>

This pings the entire rpc network which this http server is aware of. All the nodes have to be started much like this http server, then configured with  [http://ip:addr/ops/rpc/server/start](#ep04), then at least _this_ node has to be aware of all their rpc addresses with 
[http://ip:addr/ops/rpc/addrs/put](#ep01)

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

---
<div id=ep06><b>http://ip:addr/cmd/add</b></div>
This adds new vector data which can be queried later on (the "nearest" part of KNN). The endpoint can accept multiple vectors, which are distributed randomly accross the rpc network (so at least one rpc node must be known to this server, which might be itself). There are a few fail conditions for adding a vector:

- The namespace is known but the vectors there do not have the same length/dimension as the new ones. This can be checked apriori with [http://ip:addr/info/dim](#ep10)
- The total capacity of searchspaces (amount of vectors that can be added), as specified with [http://ip:addr/ops/rpc/server/start](#ep04), is exceeded with this new data. This can be mitigated apriori with [http://ip:addr/info/len](#ep11) and [http://ip:addr/info/cap](#ep12).


Also note that since this endpoint can accept multiple vectors, one has to potentially do manual batching. For instance, if a billion vectors are sent, then that might exceed the read/write deadline for this http server, which is specified when running the binary of for example cmd/simple-http-server.
  
```python
import requests

from datetime import timedelta
from datetime import timezone

# Added data can be _optionally_ expired. The time format must be accepted by
# Go, the language ddrop is written in, which i think is RFC3339, something
# like "0001-01-01T00:00:00Z".
# This will expire the data in approximately one hour.
tz = timezone.utc # Use the one that applies.
expires = datetime.now(tz) + timedelta(hours=1)
expires = expires.isoformat()

# Namespace for where to put data on each rpc node. A new namespace will
# be created if it does not already exist.
ns = ""

resp = requests.post(
  url="http://localhost:8080/cmd/add",
  # Note, there are a couple additional options (such as expiration),
  # but they are not covered in this quickstart example.
  json=[
    {"namespace": ns, "vec": [1,1,1], "expires":expires},
    {"namespace": ns, "vec": [2,2,2], "expires":expires},
    {"namespace": ns, "vec": [3,3,3], "expires":expires}
  ]
)

# Status: 200
# Json can be something like this
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': [True, True, True], # Bool status per vector.
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```
 
---
<div id=ep07><b>http://ip:addr/cmd/knn</b></div>
 
This endpoint is for doing KNN requests on top of the rpc network. As such, at least one rpc server must have been started with [http://ip:addr/ops/rpc/server/start](#ep04) and this http server must know of the rpc node through [http://ip:addr/ops/rpc/addrs/put](#ep01). Additionally, the network naturally needs to have data added with [http://ip:addr/cmd/add](#ep06).

Also note that the `ttl` field of a KNN request should naturally not exceed the read/write timeouts of this http server (that is set with a cli flag when starting the server, as with cmd/simple-http-server).


```python
import requests

resp = requests.post(
  url="http://localhost:8080/cmd/knn",
  json={
    # Multiple vecs can share the args. Note that this must be the same dimensions
    # as the vectors in the query pool for the namespace specified below.
    "queryVecs": [ [0,0,0] ],
    # Note that some of these are strict, so 
    "args": {
    	# Namespace is used to group search spaces together, based on logical
    	# meaning, but also for having uniform vector dimensions.
      "namespace": "",
      # Priority specifies how important a KNN query is -- higher is better.
	    # It influences the number of light-threads used, though not necessarily
	    # a one-to-one mapping. Must be > 0.
      "priority": 1,
      # The distance function to use. 0=Euclidean distance, 1=Cosine similarity.
      # Must be one of those two.
      "KNNMethod": 0,
	    # Ascending plays a role with ordering _and_ the meaning is dependent
	    # somewhat on the KNNMethod field.
	    # 
	    # Euclidean distance, for instance, works on the principle that lower
	    # is better, so then it would make sense to have ascending=True for
	    # KNN. For K-furthest-neighs, Ascending=False has to be used, as that
	    # would reverse the order. The exact opposite is true for Cosine simi.
      "ascending": True,
      # K is the K in KNN. However, the actual result might be less than this
	    # number, for multiple reasons. One of them is that there simply might
	    # not be enough data to search. Another reason is that the underlying
	    # knn pkg uses a few optimization tricks to trade accuracy for speed,
	    # the reamainding fields below give more documentation.
      "k":2,
      # Extent specifies the extent of a search, in a range (0, 1]. For
	    # example, 0.5 will search half the search spaces. This is used to
	    # trade accuracy for speed.
      "extent": 1.0,
      # Accept is another optimization trick; the search will be aborted
	    # when there are "k" results with better than "accept" accuracy
      # The meaning of "better" will depend on which "KNNMethod" is used.
      "accept": 1.0,
      # Reject is another optimization trick; the knn search pipeline will
	    # drop all values worse than this fairly early on, such that the
	    # load on downstream processes/pipes gets alleviated. Do note that
	    # this is evaluated before "accept", so "accept" can be cancelled
	    # out by "reject". The meaning of "better" will depend on which
      # "KNNMethod" is used.
      "reject": 9.0,
      # TTL specifies the deadline for a knn request. The distributed pipeline
      # will start shutting down for this request after the deadline. Do note
      # That this system will reject the query if it's too low, so one should
      # factor in the latency of network, queue, and estimated query.
      # This is all accessible through different endpoints, namely
      # - http://ip:addr/info/knnLatency
      # - http://ip:addr/info/knnMonitor (alternative to the above)
      # - http://ip:addr/cmd/ping
      "ttl": 1000000000, # 1 second.
      # If this is True, then metadata of the query will be registered for
      # detailed monitoring (if query gets to the pipeline). This has some
      # small performance penalty, and is as such implemented as just a
      # convenience. The endpoint for getting this data is:
      # - http://ip:addr/info/knnMonitor
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
#     # This is the k results for this query. Note that this might not actually
#     # be of lenght k, depending on amount of data in the pool, how the knn
#     # request args were specified, etc. Also note that different results might
#     # have come from different rpc nodes, so there is included some network data.
#     'results': [
#       {
#         # Addr of the rpc node who had this result.
#         'remoteAddr': 'localhost:8081',
#         # Network error.
#         'netErr': None,
#         # Result vector and result score.
#         'payload': {
#           # The vector that was found. Since this is the first object in this
#           # 'results' list, then this is the best result (according to the cfg).
#           'vec': [1, 1, 1],
#           # Distance score. We used Euclidean distance.
#           'score': 1.7320508075688772
#         },
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
  
  
---
<div id=ep08><b>http://ip:addr/info/namespaces</b></div>

This endpoint gets all used namespaces in all rpc nodes.
  
```python
import requests

resp = requests.post(
  url="http://localhost:8080/info/namespaces",
  json={}
)

# Status 200
# JSON structure:
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': ['a', 'b', 'etc'], # namespaces for this rpc node.
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```  
  
---
<div id=ep09><b>http://ip:addr/info/namespace</b></div>

This endpoint is for checking if a particular namespace exists in all rpc nodes.

```python
import requests

resp = requests.post(
  url="http://localhost:8080/info/namespace",
  json="some namespace that exists"
)

# Status 200
# JSON structure:
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': True, # indication for if the namespace exists.
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```  
  
---
<div id=ep10><b>http://ip:addr/info/dim</b></div>
 
This endpoint is for checking the vector pool dimension (length of each vector) for all rpc nodes.

```python
import requests

resp = requests.post(
  url="http://localhost:8080/info/dim",
  json="some namespace that exists"
)

# Status 200
# JSON structure:
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': {
#       'lookupOk': True,
#       'dim': 3
#     },
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```  
 
---
<div id=ep11><b>http://ip:addr/info/len</b></div>

This endpoint is for checking the number of searchspaces and vectors in a namespace for all rpc nodes.

```python
import requests

resp = requests.post(
  url="http://localhost:8080/info/len",
  json="some namespace that exists"
)

# Status 200
# JSON structure:
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': {
#       'lookupOk': True,
#       'nSearchSpaces': 1,
#       'nVecs': 3 # Total for all searchspaces for this node.
#     },
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```  

---
<div id=ep12><b>http://ip:addr/info/cap</b></div>
 
This endpoint is for checking how many search spaces that can exist in this namespace for all rpc nodes. Note that this depends on how the rpc nodes were set up.

```python
import requests

resp = requests.post(
  url="http://localhost:8080/info/cap",
  json="some namespace that exists"
)

# Status 200
# JSON structure:
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': {
#       'lookupOk': True,
#       'cap': 1000,
#     },
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```  
 
---
<div id=ep13><b>http://ip:addr/info/knnLatency</b></div>
  
This endpoint is for checking the KNN queue and query latency of a particular namespace for all rpc nodes. The length of time that is tracked is specified in [http://ip:addr/ops/rpc/server/start](#ep04), with `json["cfg"]["newLatencyTrackerArgs"]`.
  
```python
import requests

resp = requests.post(
  url="http://localhost:8080/info/knnLatency",
  json={
    # Namespace.
    "key": "",
    # How far back in time.
    "period": 1000000000 # Last second.
  }
)

# Status 200
# JSON structure:
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': {
#       'lookupOk': True, # True if namespace exists
#       'queue': 0,       # Average queue time (in nanosec) for the given period.
#       'query': 0,       # Average query time (in nanosec) for the given period.
#       'boundsOk': True  # False if given period is more than what was tracked.
#     },
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```  
  
---
<div id=ep14><b>http://ip:addr/info/knnMonitor</b></div>
  
This endpoint is for retrieving fairly detailed data about queries that were done in [http://ip:addr/cmd/knn](#ep07), using `json["args"]["monitor"]`.  The length of time that is tracked is specified in [http://ip:addr/ops/rpc/server/start](#ep04), with `json["cfg"]["newKNNMonitorArgs"]`.

```python
import requests

from datetime import timedelta
from datetime import timezone

# Monitoring data is queried with a span of time in this format:
#   {"start": now, "end": then} 
# ... where "end" specifies how far back in time to go.
# The time format is RFC3339, something like "0001-01-01T00:00:00Z".
tz = timezone.utc # Use the one that applies.
now = datetime.now(tz)
now = now.isoformat()
then = datetime.now(tz) - timedelta(hours=1)
then = then.isoformat()

resp = requests.post(
  url="http://localhost:8080/info/knnMonitor",
  json={
   "start": now,
   "end": then
  }
)

# Status 200
# JSON structure:
# [ # List since there can be multiple rpc nodes.
#   {
#     'remoteAddr': ':8081', # rpc addr for the responding node.
#     'netErr': None, # rpc network error.
#     'payload': {
#       # When tracking started.
#       'created': '0001-01-01T00:00:00Z',
#       # Tracking duration in nanoseconds.
#       'span': 0,
#       # Number of requests recorded (including fails).
#       'n': 0,
#       # Numer of (completely failed) requests.
#       'nFailed': 0,
#       # Average KNN latency for all erquests.
#       'avgLatency': 0,
#       # Average score for all requests.
#       'avgScore': 0,
#       # Same as avgScore but without fails.
#       'avgScoreNoFails': 0,
#       # Success ratio "got n / wanted k" (where k is the k in KNN). 
#       'avgSatisfaction': 0
#     },
#     'networkLatency': 419000 # http->rpc server latency in nanoseconds.
#   }
# ]
print(resp, resp.json())
```  



