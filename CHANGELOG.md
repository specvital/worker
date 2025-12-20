# Changelog

## [1.0.6](https://github.com/specvital/collector/compare/v1.0.5...v1.0.6) (2025-12-19)

### 🎯 Highlights

#### 🐛 Bug Fixes

- resolve 60-second timeout failure during large analysis jobs ([ed18bc3](https://github.com/specvital/collector/commit/ed18bc3f587c0a446ea41b09431126ab1d22bba5))

## [1.0.5](https://github.com/specvital/collector/compare/v1.0.4...v1.0.5) (2025-12-19)

### 🎯 Highlights

#### 🐛 Bug Fixes

- resolve 60-second timeout issue during bulk INSERT operations on NeonDB ([0b6bc9b](https://github.com/specvital/collector/commit/0b6bc9bbef0c14190ec953a14caa5b29da0422d5))

## [1.0.4](https://github.com/specvital/collector/compare/v1.0.3...v1.0.4) (2025-12-19)

### 🔧 Maintenance

#### ♻️ Refactoring

- migrate queue system from asynq(Redis) to river(PostgreSQL) ([9664002](https://github.com/specvital/collector/commit/9664002057ef1f801dd8313e9f081760c3e0af21))

#### 🔨 Chore

- missing changes ([de9c0ec](https://github.com/specvital/collector/commit/de9c0ecaa7e136fe71d70a4a231dbd194ed7a33d))

## [1.0.3](https://github.com/specvital/collector/compare/v1.0.2...v1.0.3) (2025-12-18)

### 🎯 Highlights

#### 🐛 Bug Fixes

- fix git clone failure in runtime container ([f11dfa3](https://github.com/specvital/collector/commit/f11dfa3e4090a6412af71b58a7eca6f081e49d4d))

### 🔧 Maintenance

#### ♻️ Refactoring

- remove unused dead code ([95cee17](https://github.com/specvital/collector/commit/95cee17e1307fe3e0cc23ba7d549292b05c19744))

#### 🔨 Chore

- sync docs ([9007e97](https://github.com/specvital/collector/commit/9007e97bbff365afcae69cfcba2f501732a20c8b))

## [1.0.2](https://github.com/specvital/collector/compare/v1.0.1...v1.0.2) (2025-12-17)

### 🔧 Maintenance

#### 🔧 Internal Fixes

- fix asynq logs incorrectly classified as error in Railway ([d2180cc](https://github.com/specvital/collector/commit/d2180cc1182a0f1187f7dd63b982fc7816e3be47))

## [1.0.1](https://github.com/specvital/collector/compare/v1.0.0...v1.0.1) (2025-12-17)

### 🎯 Highlights

#### 🐛 Bug Fixes

- enable CGO for go-tree-sitter build ([50b1fea](https://github.com/specvital/collector/commit/50b1fea3c7834bd585c3a23615d8acb5cbae8a5f))

## [1.0.0](https://github.com/specvital/collector/releases/tag/v1.0.0) (2025-12-17)

### 🎯 Highlights

#### ✨ Features

- add adaptive decay logic for auto-refresh scheduling ([8a85854](https://github.com/specvital/collector/commit/8a858547c7b8a5190176253830b862588cda8042))
- add enqueue CLI tool ([5697cb9](https://github.com/specvital/collector/commit/5697cb9533b8dcfd8c7c90fa34d5419b181fa287))
- add focused/xfail to test_status and support modifier column ([cd60233](https://github.com/specvital/collector/commit/cd602333aa9eba34e8a3b21bbcd91040bfe59936))
- add job timeout to prevent long-running analysis jobs ([392b43e](https://github.com/specvital/collector/commit/392b43e7395aa923fd05a96872e5f0c8911a8845))
- add local development mode support to justfile ([2ca2d51](https://github.com/specvital/collector/commit/2ca2d51337cd8c425e54a5f3ffd257bde28e403c))
- add local development services to devcontainer ([a30ca9e](https://github.com/specvital/collector/commit/a30ca9eca3df2aa4a418d94b1525f8ac170ee6c1))
- add OAuth token parameter to VCS Clone interface ([de518d0](https://github.com/specvital/collector/commit/de518d05eb534c97cf5e647168ff9d8a686ae00e))
- add semaphore to limit concurrent git clones ([9ddbc06](https://github.com/specvital/collector/commit/9ddbc06291c8fb41e0e39def7f32edaa5830f1ba))
- add UserRepository for OAuth token lookup ([9a16ec1](https://github.com/specvital/collector/commit/9a16ec1f37031d15612198b27f5316d4ab066225))
- implement analysis pipeline (git clone → parse → DB save) ([66dd262](https://github.com/specvital/collector/commit/66dd2627b6e7d5b46ab7bc4358a1f6178d77cfee))
- implement asynq-based worker basic structure ([4dd16ad](https://github.com/specvital/collector/commit/4dd16ad22b355229f6c9db12e076427f7ff0c2ea))
- initialize collector service project ([1d3c8cf](https://github.com/specvital/collector/commit/1d3c8cf3a570a719301fa953c9cccb9c53a0358a))
- integrate OAuth token lookup logic into UseCase ([cb3f911](https://github.com/specvital/collector/commit/cb3f9114f89b9435f037379b6eac07c51dc53d96))
- integrate scheduler for automatic codebase refresh ([e0a1a15](https://github.com/specvital/collector/commit/e0a1a15dac6ef26b62debf1d3b52d172cf8d8ed6))
- record failure status in DB when analysis fails ([6485ac3](https://github.com/specvital/collector/commit/6485ac345c9562404e435ea32962a1b90a13fd5b))
- support external analysis ID for record creation ([5448202](https://github.com/specvital/collector/commit/54482021c84b6faa466f0ce006de06ddcd79d22d))
- support OAuth token decryption for private repo analysis ([8d0ad30](https://github.com/specvital/collector/commit/8d0ad307ce07aa7562daa6a38230e19ce2cc1644))

#### 🐛 Bug Fixes

- handle missing error logging and DB status update on analysis task failure ([64ae8d9](https://github.com/specvital/collector/commit/64ae8d9fcbc620e7470e2c7cc21ade39d7327f8d))
- parser scan failing due to unexported method type assertion ([6256673](https://github.com/specvital/collector/commit/6256673dc23d48998851f977fcd034498c591642))
- remove unnecessary wait and potential deadlock in graceful shutdown ([b78c981](https://github.com/specvital/collector/commit/b78c981c662d32fde66c0890da0e226e9b4a4d3e))

### 🔧 Maintenance

#### 🔧 Internal Fixes

- go mod tidy ([c58f73b](https://github.com/specvital/collector/commit/c58f73b40f2de49c8c69b1d67efd45b1487c0359))

#### 💄 Styles

- format code ([5e994e2](https://github.com/specvital/collector/commit/5e994e2ab90f6ae6a8cd64d392b946c9bde0bd1d))

#### ♻️ Refactoring

- centralize dependency wiring with DI container ([c1b8215](https://github.com/specvital/collector/commit/c1b82151bdba8b62e194100e6f04271fd3f4e026))
- extract domain layer with zero infrastructure dependencies ([7ba9e51](https://github.com/specvital/collector/commit/7ba9e51a0ffa76327736e91c828fe5949cbfbcb6))
- extract repository layer from AnalyzeHandler ([464ecfa](https://github.com/specvital/collector/commit/464ecfa6087d91d4d399a7fee032ed2a9109a151))
- extract service layer from AnalyzeHandler ([d9faf20](https://github.com/specvital/collector/commit/d9faf200da77b3097417cd1671a3a1c5fbc5fe06))
- implement handler layer and clean up legacy packages ([23a093f](https://github.com/specvital/collector/commit/23a093f6d5558d10decb21272289c1d99e583101))
- implement repository adapter layer (Clean Architecture Commit 3) ([8b0e433](https://github.com/specvital/collector/commit/8b0e43372fec1b15fa7e7d76794be37d42b6988e))
- implement use case layer with dependency injection ([b2be6ff](https://github.com/specvital/collector/commit/b2be6ff3d3a1c3662f816958d614f4a89215aba6))
- implement VCS and parser adapter layer (Clean Architecture Commit 4) ([1b2e34f](https://github.com/specvital/collector/commit/1b2e34f61665d6513c2654712390b67524dfd731))
- move infrastructure packages to internal/infra ([6cc1d1c](https://github.com/specvital/collector/commit/6cc1d1caf8722317967eae5c2de6c40c71467ce2))
- separate Scheduler from Worker into independent service ([9481141](https://github.com/specvital/collector/commit/9481141e99f0adcc225e93d05c8104846f836c17))
- split entry points for Railway separate deployments ([d899192](https://github.com/specvital/collector/commit/d899192cb1772fe9ed16d426d460e016c1bbf2ee))

#### ✅ Tests

- add AnalyzeHandler unit tests ([0286e7c](https://github.com/specvital/collector/commit/0286e7cd687d65be853e503a883cd74010f8dede))
- remove unnecessary skipped tests ([f8c0eb4](https://github.com/specvital/collector/commit/f8c0eb40a4f99252650bbf9e0f9ca93a378223fb))

#### 🔧 CI/CD

- configure semantic-release automated deployment pipeline ([37f128f](https://github.com/specvital/collector/commit/37f128f2c9d113144d2af530e88f84d3209f235c))

#### 🔨 Chore

- add bootstrap command ([c8371f0](https://github.com/specvital/collector/commit/c8371f0d5f47a19353c260ecf83c5033b4e5ba53))
- add Dockerfile for collector service ([6e3b0e4](https://github.com/specvital/collector/commit/6e3b0e4225b4b0875e2ee0bae909a594b1b9f87c))
- add example env file ([64a24a4](https://github.com/specvital/collector/commit/64a24a4de88aa5e4954a59b049b915c2012da79e))
- add gitignore item ([8fc64a6](https://github.com/specvital/collector/commit/8fc64a6ab0a3abca1cf1d73f458adf17bb752ced))
- add migrate local command ([baabcfe](https://github.com/specvital/collector/commit/baabcfe97f3905122026a752ea2ba7f7ed07917b))
- add PostgreSQL connection and sqlc configuration ([eecc4a6](https://github.com/specvital/collector/commit/eecc4a69a8b8c6e5c67a8f652a97ae784ecca1c1))
- add useful action buttons ([02fa778](https://github.com/specvital/collector/commit/02fa7785ac4ba1505e06ac3add60621cf01d1be9))
- adding recommended extensions ([30d5d0b](https://github.com/specvital/collector/commit/30d5d0b0fccc3190313456433e24e1342c18d641))
- ai-config-toolkit sync ([3091cf4](https://github.com/specvital/collector/commit/3091cf46ca2e6a24f5c299fe4f8008659fe1b8c8))
- ai-config-toolkit sync ([decf96b](https://github.com/specvital/collector/commit/decf96b2c47b278ef56a3e76c6174c9688f883c3))
- delete file ([f48005c](https://github.com/specvital/collector/commit/f48005cad322fa2586e9bf315e2bce3c608dcd8b))
- dump schema ([b90bab0](https://github.com/specvital/collector/commit/b90bab0f0c33d52683b6ff6a1f132702eb54a077))
- dump schema ([370409c](https://github.com/specvital/collector/commit/370409cee67512bc3f21ac3f5835357303db9b57))
- dump schema ([d704305](https://github.com/specvital/collector/commit/d7043054a0a59ac755bc23a85a4fd39f5ce97a0a))
- Global document synchronization ([cead255](https://github.com/specvital/collector/commit/cead255f25f48397848d55cf1417f21466dae67c))
- sync ai-config-toolkit ([e559889](https://github.com/specvital/collector/commit/e55988903526ade4630d2d6516e67ad1354ff67e))
- update core ([d358131](https://github.com/specvital/collector/commit/d358131e3e6197ee8958655b3cc1cfa7d0ed9ca6))
- update core ([b47592e](https://github.com/specvital/collector/commit/b47592e2a6668c25585d0338099e83e7b72bf1d5))
- update core ([395930a](https://github.com/specvital/collector/commit/395930a21bb48b8283cac037cde1999e44ae69c6))
- update schema.sql path in justfile ([0bcbe79](https://github.com/specvital/collector/commit/0bcbe794cdbccff58e2babe75a6308aacc6ad5d0))
- update-core version ([cc65b03](https://github.com/specvital/collector/commit/cc65b0325a1e828e24270753d76fa91ff01eeb45))
