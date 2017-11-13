# BuildKit BoF at [Moby Summit Copenhagen (October 2017)](https://blog.mobyproject.org/dockercon-eu-moby-summit-recap-moby-project-security-and-networking-16da4f8172f0)

## Unformatted and INACCURATE note

```
10-20 attendees: me(akihiro suda), tonis, vincent demeester, mhbauer, simon, tibor, sebastiaan, sven…

Q&A:
	- Distributed mode? [vincent, et al]
		○ Single graph solver, single state for instruction cache [tonis]
		○ Stateless masters and workers [akihiro]
		○ Cpu stat… [tonis]
	- Infinit for distributed cache? [simon]
		○ Docker registry? IPFS? [akihiro]
		○ Needs investigation for plugin infrastructure
	- Multi-output for multi-arch?
		○ Execute requests in parallel and caches are used, so it produces multi outputs [tonis]
	- How frontend is implemented?
		○ Dockerfile2llb and frontend cmd
	- Gobuild in dockerfile? [simon]
		○ Not at the moment, we need to add Dockerfile instruction [tonis]
		○ Like: FROM … AS gobuild FROM alpine RUN --nested=gobuild github.com/cmd/foo > foo
	- Buildkit for CI [simon]
	- Serverless buildkit
	- Direct LLB build for Dockerfile?
	- Why client and server?
		○ For moby
	- Ccache
		○ Persistent source [tonis]
	- Compare with bazel [tibor]
	- Image validation, notary? [vincent]
	- Libentitlements for buildkit? [tonis]
		○ Ctrd-level entitlements? [tibor]
	- Rootless containers? Although unlikely to work with apt/yum [akihiro]
		○ Need to find solution that works [tonis]
	- Integration to moby asm [tibor]
		○ For building vmlinuz? [akihiro]
	- Cache format [tibor]
		○ Ctrd snapshot but more coupled with content [tonis]
	- Delta image as in balena [tibor]
		○ As ctrd snapshotter and differ [tibor, akihiro]
	- Two types of cache (llb, content) [tonis]
	- Windows support [simon]
		○ Wait for ctrd 1.1 [tonis]
		○ Needs worker constraint metadata for vertex [akihiro]
		○ Windows is not so fast.
	- Multi-arch
		○ Worker constraint vertex md
		○ Copier
		○ Manifest list generator
```
