# Changelog

## [1.0.1](https://github.com/GauranshMathur/ARR_MCP/compare/v1.0.0...v1.0.1) (2026-08-16)


### Bug Fixes

* **server:** report the real version for source installs ([7590939](https://github.com/GauranshMathur/ARR_MCP/commit/7590939d0e7c2b9fe353f59faa88566d9ddb2b06))
* **server:** report the real version for source installs ([021836c](https://github.com/GauranshMathur/ARR_MCP/commit/021836ccdaeaeb4c528b4fd5255f5ceaf804785f))

## [1.0.0](https://github.com/GauranshMathur/ARR_MCP/compare/v0.3.0...v1.0.0) (2026-08-16)


### ⚠ BREAKING CHANGES

* the command package moved from ./cmd/server to ./cmd/arr-mcp. Update `go install github.com/GauranshMathur/ARR_MCP/cmd/arr-mcp@latest` and `go build -o arr-mcp ./cmd/arr-mcp`. Users of the container image are unaffected -- the entrypoint and every flag are unchanged.

### Features

* rename cmd/server to cmd/arr-mcp ([a9519a6](https://github.com/GauranshMathur/ARR_MCP/commit/a9519a6687f8cfe4403c1eeb28b88f8311d02ee9))

## [0.3.0](https://github.com/GauranshMathur/ARR_MCP/compare/v0.2.0...v0.3.0) (2026-08-16)


### Features

* **arr:** add custom format, profile and connection tools ([d6c91f1](https://github.com/GauranshMathur/ARR_MCP/commit/d6c91f1a5fdb1e269cf1508fac7bf3a2de537137))
* **arr:** add media file listing, deletion and rename previews ([c5e67a7](https://github.com/GauranshMathur/ARR_MCP/commit/c5e67a71516bb364a270df088c0a2926d908eb8d))
* **arr:** add monitoring and bulk edit control ([c00c736](https://github.com/GauranshMathur/ARR_MCP/commit/c00c736197413109195f1b2d53dd27675f8beed1))
* **arr:** add tag management for Sonarr and Radarr ([2a99bed](https://github.com/GauranshMathur/ARR_MCP/commit/2a99bedcd0a9e83511d0e3775f59d0e70fc078e1))
* **arr:** add wanted lists, blocklist, targeted search and system views ([07972b6](https://github.com/GauranshMathur/ARR_MCP/commit/07972b673b64ac8aaf9bd532764ff9d35057e76e))
* **arr:** near-complete Sonarr and Radarr API coverage ([dfa845b](https://github.com/GauranshMathur/ARR_MCP/commit/dfa845b8c62ffac7c39c5ecc5a15dcd366d34e32))
* **bazarr:** add subtitle management tools ([9cc4393](https://github.com/GauranshMathur/ARR_MCP/commit/9cc4393af3d2345ec540dd46585ca3c397a38227))
* **bazarr:** add subtitle management tools ([0a03c63](https://github.com/GauranshMathur/ARR_MCP/commit/0a03c6326f8324cf0e32d90201d661f474113240))


### Bug Fixes

* **bazarr:** correct API assumptions surfaced in code review ([38f7e66](https://github.com/GauranshMathur/ARR_MCP/commit/38f7e66b8626eeb650380fa46d250680e098ab4b))


### Refactoring

* **server:** rename the subtitle provider wrapper ([5fab81f](https://github.com/GauranshMathur/ARR_MCP/commit/5fab81f999e22004d0f343f876dbe36a33897b41))


### Documentation

* add per-client MCP setup guide ([2c20e61](https://github.com/GauranshMathur/ARR_MCP/commit/2c20e619d533bc0b4e014f1eeed355da1241addb))
* clarify the two configuration paths in the templates ([ce90a71](https://github.com/GauranshMathur/ARR_MCP/commit/ce90a71093c2cbc444099da4464d1a0a525d645a))
* correct the tool count to 50 ([a663722](https://github.com/GauranshMathur/ARR_MCP/commit/a6637220059e09b5f18fda484a3706ce1214ef46))
* document the expanded Sonarr and Radarr tool surface ([4cc49e7](https://github.com/GauranshMathur/ARR_MCP/commit/4cc49e7ed652417dbacd81dbb7f591bf0521dc36))
* per-client setup guide, quickstart and Kubernetes manifests ([c51e3d8](https://github.com/GauranshMathur/ARR_MCP/commit/c51e3d8af1aba064d5ffe8eb28f2ba53d66fa861))
* restructure README around a 60-second quickstart ([67873b6](https://github.com/GauranshMathur/ARR_MCP/commit/67873b6efaea43e88aac13b540b6d68cd204f01f))


### Build & Packaging

* add Kubernetes manifests ([d19f561](https://github.com/GauranshMathur/ARR_MCP/commit/d19f561c8753f481ff384eed29fffc1a4fbeeb33))
* document the compose deployment's constraints ([34b1f45](https://github.com/GauranshMathur/ARR_MCP/commit/34b1f450fc85c4b1965c9bee656e6a954609e5c4))

## [0.2.0](https://github.com/GauranshMathur/ARR_MCP/compare/v0.1.0...v0.2.0) (2026-08-16)


### Features

* **arr:** generalize the client with per-service API specs ([a96d25b](https://github.com/GauranshMathur/ARR_MCP/commit/a96d25baacfa0a08e8067fc13668bc03a0f87bff))
* **config:** add multi-instance configuration and permission policy ([81f1657](https://github.com/GauranshMathur/ARR_MCP/commit/81f1657b48d7fbcb037b785dc15f2ea77501f593))
* **mcp:** serve real MCP over stdio and streamable HTTP ([c696256](https://github.com/GauranshMathur/ARR_MCP/commit/c696256286ee2ce594defb9fc451f07a828815e0))
* rewrite as a real MCP server with multi-instance support ([84a01e0](https://github.com/GauranshMathur/ARR_MCP/commit/84a01e0571c461d367658ba9725f841a210095f3))


### Bug Fixes

* **arr:** satisfy errcheck and staticcheck ([9bd3182](https://github.com/GauranshMathur/ARR_MCP/commit/9bd31827c55a01ffcc37f6467469b28d52bf1894))
* **ci:** drop invalid bootstrap-sha from release-please config ([667d8df](https://github.com/GauranshMathur/ARR_MCP/commit/667d8dfed9cfd695bf119e995947894667231e11))
* **deps:** upgrade MCP SDK to v1.7.0 for CVE-2026-34742 ([f232ca8](https://github.com/GauranshMathur/ARR_MCP/commit/f232ca855266169e796f45a1e6551dc9b43c2af2))
* **logger:** write to stderr and drop redeclared package functions ([34dbca2](https://github.com/GauranshMathur/ARR_MCP/commit/34dbca2b20aad23297d24a671936c43a00c29725))
* release-please config and doubled brackets in startup log ([049d65f](https://github.com/GauranshMathur/ARR_MCP/commit/049d65f9b89e50510b8dfd9b0db99db6faa728dc))
* **security:** suppress two gosec false positives with justification ([33f83cb](https://github.com/GauranshMathur/ARR_MCP/commit/33f83cb47aceea649e7cc38d1a9183d8166990f7))
* **server:** print instance names without doubled brackets ([92fe9f3](https://github.com/GauranshMathur/ARR_MCP/commit/92fe9f3187227fcba69a903d53c1dc1d69ab1143))


### Documentation

* add AGENTS.md with repository conventions ([8d5fc45](https://github.com/GauranshMathur/ARR_MCP/commit/8d5fc4568ae59b2ffc8560a39a7106806b846827))
* limit scope to *arr-named services ([93ea704](https://github.com/GauranshMathur/ARR_MCP/commit/93ea7042ea3f9f83b1a45fbff018361d7319f2a6))
* record why download clients are out of scope ([5521a7e](https://github.com/GauranshMathur/ARR_MCP/commit/5521a7e38b7b17e5e06c367e89564b66452bb682))
* rewrite README and remove stale documentation ([8967dfd](https://github.com/GauranshMathur/ARR_MCP/commit/8967dfd2bf8973478ad7ac0ed88595820586ab67))


### Build & Packaging

* **deps:** rename module and adopt the official MCP Go SDK ([f49a17a](https://github.com/GauranshMathur/ARR_MCP/commit/f49a17a066c4322ea34e0a34fb21dc39bdfb3230))
* **docker:** add distroless image and compose quickstart ([f6002ff](https://github.com/GauranshMathur/ARR_MCP/commit/f6002ff236222e9fdb33a957a540294dffefcf54))
