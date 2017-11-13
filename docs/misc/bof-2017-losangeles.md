# BuildKit BoF at [Moby Summit Los Angeles (September 2017)](https://blog.mobyproject.org/moby-summit-los-angeles-recap-a41e6acf81f8)

## Unformatted and INACCURATE note

```
	• BuildKit BOF: [me(Akihiro Suda), Tonis, Tibor, (Patrick), (Mhbauer) (Kiril)]
		○ Gobuilder looks good workload for distributed mode [me]
			§ It depends on data sharing bottleneck, maybe filegrain [tonis]
				□ Filegrain is a server? [tibor]
					® No, client [me]
				□ Use worker node as if registry [tonis]
				□ Differs from IPFS? [tibor]
					® OCI-compatible [tonis]
					® Filegrain over IPFS is also possible [me]
		○ Usecase of nested build? [tibor]
			§ Ex. Use my local runc for docker development [tonis]
		○ Scheduling: select a host which already has the cache[tonis]
		○ Need for etcd? [me]
			§ No [tonis]
				□ HA is another problem, lower priority
		○ Cache/metadata will container worker info
		○ Solver/state.go volatile [tonis]
		○ Single buildd manager that receives build requests, workers are in worker mode [tonis]
		○ Worker could be started by passing the master node IP address [tonis]
		○ op.Run() returns its worker ID
		○ Map[CacheKey]WorkerID
		○ For root node of the graph (source, aka nodes without input), probably query ask workers whether they have a cachekey for the operation definition (pb.Op)
			§ Query: "are you able to reproduce a cachekey for this operation definition?"
				□ If none have it, choose randomly
				□ if query is heavy, query to git maybe
				□ gossip?
				□ can support many nodes
				
		○ Because of freshness: always ping original repo (at least for git)
			§ Funny corner case: cannot have 160char hex-named branch
		○ Cache for http source:  uses etags to know if it should repull, otherwise calculate hash of pulled content
		○ Local files: difficult
			§ Find the same worker that already received the content from the client the first time? Or always sync to master?
				□ Use local workre on master node for simple tasks? (with caveat of constraints)
		○ Private images
			§ Worker needs to ask master for the credential

		○ LLB
			§ not Definition bytes; send struct{Definition bytes, metadata} as a single structure object
				□ Solver/solver.go: Solve(context.Context, struct{Definition [][]bytes, Metadata map[digest]MetadataEntry}) 
llb.Marshal should also return this struct object
```
