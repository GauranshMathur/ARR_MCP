# Changelog

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
