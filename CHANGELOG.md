# Changelog

## [1.2.2](https://github.com/specvital/worker/compare/v1.2.1...v1.2.2) (2026-02-02)

### 🔧 Maintenance

#### 🔧 Internal Fixes

- **deploy:** apply railway.json config during CLI deployment ([f0f5e79](https://github.com/specvital/worker/commit/f0f5e791ecd897ba692b49a03b8662f97a5be982))

#### ♻️ Refactoring

- **deploy:** move Railway config files to infra/railway/ ([bcd7a87](https://github.com/specvital/worker/commit/bcd7a87aeff8d968397a8115dcae313c3173d1d8))

## [1.2.1](https://github.com/specvital/worker/compare/v1.2.0...v1.2.1) (2026-02-02)

### 🔧 Maintenance

#### 🔧 Internal Fixes

- **deploy:** remove railway link command for Project Token usage ([50e4ac1](https://github.com/specvital/worker/commit/50e4ac1be248860f5a08ea88f48f4f12ef50f4df))

#### ♻️ Refactoring

- **deploy:** reorder workflow to run release after successful deployment ([404b956](https://github.com/specvital/worker/commit/404b95676af528a62449b9712958a261304c732e))

#### 🔨 Chore

- add railway cli ([98bf945](https://github.com/specvital/worker/commit/98bf945d2aee5952d7e7da41554f805869b3820b))

## [1.2.0](https://github.com/specvital/worker/compare/v1.1.1...v1.2.0) (2026-02-02)

### 🎯 Highlights

#### ✨ Features

- **analysis:** add usage_events recording on analysis completion ([0593683](https://github.com/specvital/worker/commit/05936838603fb478da028d87228001bab21c48a5))
- **analysis:** store parser_version in analysis records ([aa47dab](https://github.com/specvital/worker/commit/aa47dab02fc13401415ec0bd7485e0c4ba473b48))
- **autorefresh:** add parser_version comparison to auto-refresh conditions ([4cc6cc4](https://github.com/specvital/worker/commit/4cc6cc4323a87d1702448bb3c94a42af2a10cab2))
- **batch:** implement automatic chunking for large test sets ([cf7c691](https://github.com/specvital/worker/commit/cf7c6914ad71273c5e783f7a94b812be2edba3a9))
- **bootstrap:** add multi-queue subscription support ([dc4c254](https://github.com/specvital/worker/commit/dc4c254d2194a5faa9d710452ddc652d376e040d))
- **bootstrap:** register parser version to system_config on Worker startup ([290a5ef](https://github.com/specvital/worker/commit/290a5efa4e3edf928c68fb9e08d1e6ea41380f77))
- **config:** add feature flag infrastructure for Phase 1 V2 architecture ([9c1c4c9](https://github.com/specvital/worker/commit/9c1c4c9e6fa5aab1558bd490c3cc0f43263f83b4))
- **db:** add SQLc queries and domain model for test_files table ([ca530c4](https://github.com/specvital/worker/commit/ca530c4162a79b589b2945b7d58b4fdcf5781f72))
- **deploy:** add retention-cleanup service Railway configuration ([876c3f3](https://github.com/specvital/worker/commit/876c3f37b14cc0c7bf041ded6c197b0c619f70b7))
- **deploy:** migrate to Railway IaC with GitHub Actions deployment ([b842318](https://github.com/specvital/worker/commit/b842318fe5a3f6d7c56780dd93979299382030e4))
- **gemini:** add auto-recovery for missing test indices from AI output ([c3173e8](https://github.com/specvital/worker/commit/c3173e8a08fcbaca230463ec26f9767b88a5ee32))
- **gemini:** add domain description field to v3BatchResult ([62d5b8d](https://github.com/specvital/worker/commit/62d5b8d6894ef9ade164af83315b41dcfe8f054a))
- **gemini:** add Phase 1 V3 sequential batch architecture feature flag ([71fc07e](https://github.com/specvital/worker/commit/71fc07eb4e3f0fd4f8538167083e1325592f53fa))
- **gemini:** add Stage 1 Taxonomy cache ([7ccea47](https://github.com/specvital/worker/commit/7ccea4782b024e935caeb7bd4c7fbb7b10a4ec3e))
- **gemini:** add V3 prompt templates for order-based batch classification ([b186601](https://github.com/specvital/worker/commit/b1866013a2368ed12365e6d52c3907ad657e27d2))
- **gemini:** implement Batch API polling and result parsing ([66d7ddf](https://github.com/specvital/worker/commit/66d7ddf4e2bcb13eadfcc0e1bc0c8851d83da28a))
- **gemini:** implement Batch API Provider base structure ([7214cbd](https://github.com/specvital/worker/commit/7214cbd4e647f600739550708b0a0e660cd9b418))
- **gemini:** implement Phase 1 V2 Orchestrator ([f2f453b](https://github.com/specvital/worker/commit/f2f453b0fd8edd3720ca9fb44161705006676460))
- **gemini:** implement Phase1 quality metrics collector ([5aef0df](https://github.com/specvital/worker/commit/5aef0dfa62ea17769a63a330ed4f066d8b9e679e))
- **gemini:** implement Phase1PostProcessor for classification validation and normalization ([c7fee4e](https://github.com/specvital/worker/commit/c7fee4e241d5deebac4a0d51dad65bfc03985779))
- **gemini:** implement Stage 1 Taxonomy extraction logic ([be9bf50](https://github.com/specvital/worker/commit/be9bf503514e87783edd80a9016f2145bce96a52))
- **gemini:** implement Stage 2 Test Assignment logic ([531962b](https://github.com/specvital/worker/commit/531962bc38dd771af6b9d5dd4ec1bdcc16d759d3))
- **gemini:** implement V3 batch processing core logic with response validation ([c528346](https://github.com/specvital/worker/commit/c5283462c699e1c449d36bee9b689b15a0b19dcb))
- **gemini:** implement V3 batch retry logic with split and individual fallback ([0945156](https://github.com/specvital/worker/commit/094515633c8405ad8db358406561ce457231eefd))
- **gemini:** implement V3 orchestrator for sequential batch processing ([1df43b2](https://github.com/specvital/worker/commit/1df43b2773997d4d4e499c1ff4daaef6e008723d))
- **gemini:** integrate Phase 1 V2 feature flag router ([cf72f02](https://github.com/specvital/worker/commit/cf72f024894573d6462d13f1a82c18b7be3efe62))
- **gemini:** integrate quality components into V3 orchestrator ([513232b](https://github.com/specvital/worker/commit/513232b174e31e113d6537bc1267282f73074da2))
- **mapping:** add Core DomainHints to Domain model conversion ([570e07a](https://github.com/specvital/worker/commit/570e07ab788660352dcb2444cafb3942e606b046))
- **mock:** add response delay simulation to mock AI provider ([78aa6c4](https://github.com/specvital/worker/commit/78aa6c400478a72c1ca28cdd87ea0f2aa135cbb6))
- **prompt:** implement Stage 1 Taxonomy extraction prompt ([eb316c3](https://github.com/specvital/worker/commit/eb316c325e653a6df9b5f2c2c1c07e6969b2b318))
- **prompt:** implement Stage 2 Assignment prompt ([c9775d1](https://github.com/specvital/worker/commit/c9775d1b9c1d96383021e0115a6f6132884d6553))
- **queue:** add fairness middleware for per-user job limiting ([620849f](https://github.com/specvital/worker/commit/620849f7f212e02138f719fb061c9ec1e89454a2))
- **queue:** add multi-queue configuration support for River server ([71274b4](https://github.com/specvital/worker/commit/71274b4a414ee2dc68be6e0f62d3b41355629f82))
- **queue:** add per-user concurrent job limiting ([421904c](https://github.com/specvital/worker/commit/421904c4f4b0dd3732a18a990ae867da91deebad))
- **queue:** add priority queue name constants and environment config ([9a6df10](https://github.com/specvital/worker/commit/9a6df10743bdad5dde74aa394f39ace2fad0c0f6))
- **queue:** implement userID and tier extraction from River jobs ([b14baf5](https://github.com/specvital/worker/commit/b14baf5e91414cfd38a86b4800b51951111836d9))
- **queue:** integrate Batch mode into SpecView Worker ([5cb5155](https://github.com/specvital/worker/commit/5cb5155601f032d8cdc1c7ed1822c992147e31fd))
- **queue:** integrate fairness middleware into River server ([2206b19](https://github.com/specvital/worker/commit/2206b192c6ce0c008b7a0450c4fba6d7a67e63cc))
- **quota:** release quota reservation on job completion/failure in Worker ([8c69599](https://github.com/specvital/worker/commit/8c695993816871eac19ffde14961a02fed540ac7))
- **repository:** add system_config repository and parser version extraction ([72bd468](https://github.com/specvital/worker/commit/72bd4681216d5dffefba222d94f62c6cc60799b0))
- **retention:** add bootstrap and entry point for retention cleanup ([6e03a7f](https://github.com/specvital/worker/commit/6e03a7f81829b310d43a5c0c575f066fac20ec1f))
- **retention:** add domain and usecase layer for retention cleanup ([5e8c05a](https://github.com/specvital/worker/commit/5e8c05ae44ae5f263af7d0466c239959bffb1612))
- **retention:** add repository layer for retention-based cleanup ([792f106](https://github.com/specvital/worker/commit/792f106303968946ecd8ebcb22a72b8dccdb9fb5))
- **retention:** store retention_days_at_creation snapshot on record creation ([878d87c](https://github.com/specvital/worker/commit/878d87c9fde72ee44f2b0f61b7dd56e3b81dc3c0))
- **scheduler:** add dedicated queue support for auto-refresh jobs ([ba91bd9](https://github.com/specvital/worker/commit/ba91bd994ed3914a767eea0d092961263e2a9972))
- **specgen:** add MOCK_MODE env var for spec document generation without AI calls ([364dfda](https://github.com/specvital/worker/commit/364dfdab35f0d5b7ca67969791594340fadc8817))
- **specview:** add analysis_id and ETA to Phase 2 logs ([f568df6](https://github.com/specvital/worker/commit/f568df690b523b3c9953378c9c02b01145f225bf))
- **specview:** add Batch API routing logic to UseCase ([36560f3](https://github.com/specvital/worker/commit/36560f3bbb63cc217056760d291c9ecab168f3a9))
- **specview:** add behavior cache stats to SpecViewResult ([a70fa07](https://github.com/specvital/worker/commit/a70fa073ee72eea6be19bb7571f317ff9da870a6))
- **specview:** add behavior cache types and interfaces ([a871845](https://github.com/specvital/worker/commit/a871845be14de5e8908adafd22d7ce5879df4568))
- **specview:** add domain layer foundation for SPEC-VIEW Level 2 ([3644739](https://github.com/specvital/worker/commit/3644739c794512b912f221a3e0184080570e4fda))
- **specview:** add domain types for Phase 1 V2 two-stage architecture ([cc75bb5](https://github.com/specvital/worker/commit/cc75bb5a685a48969a95a8127bb7ffe720cd5faf))
- **specview:** add force regenerate option ([09821f9](https://github.com/specvital/worker/commit/09821f9f278507409880690579603406a855b913))
- **specview:** add Phase 1 chunking for large test repositories ([8831021](https://github.com/specvital/worker/commit/88310213b6b563e79c39d34b26571c7db55b8288))
- **specview:** add Phase 1 classification cache foundation types ([42a2a47](https://github.com/specvital/worker/commit/42a2a475480e3f9122c695011cc5154754899416))
- **specview:** add Phase 2 progress logging for job monitoring ([bfbb938](https://github.com/specvital/worker/commit/bfbb9384364fb2254de3d30c7e7e35401e9562ec))
- **specview:** add Phase 3 executive summary generation ([8ed1d84](https://github.com/specvital/worker/commit/8ed1d84f681be210d315d3cb6b3770a9d50c4222))
- **specview:** add phase timing logs for performance monitoring ([a04dc43](https://github.com/specvital/worker/commit/a04dc433b5f18d261d4ad8fda8c3b20755fe6d5d))
- **specview:** add placement AI adapter for incremental cache ([1cbefd6](https://github.com/specvital/worker/commit/1cbefd631b39218863b393c5a84ca6fdb425ed1d))
- **specview:** add real-time Gemini API token usage tracking ([109d572](https://github.com/specvital/worker/commit/109d57295ea501bd6a9a7179038a25584a64ff69))
- **specview:** add repository context (owner/repo) to logs ([ef57a38](https://github.com/specvital/worker/commit/ef57a38cd18507862ad4d4cc0e8830c0165f9d34))
- **specview:** add retry info and phase context to failure logs ([87a1993](https://github.com/specvital/worker/commit/87a1993a2aa275c7ab241e81e7dbc3210de9c72e))
- **specview:** add test diff calculation for incremental cache ([b61b78b](https://github.com/specvital/worker/commit/b61b78b26bd8c2353e448a3a0d3abff002b8b395))
- **specview:** add uncategorized handling for incremental cache ([b6b3ea5](https://github.com/specvital/worker/commit/b6b3ea5e78a5329148a21489b7bb9de3631d7b4a))
- **specview:** add user_specview_history recording ([abc148b](https://github.com/specvital/worker/commit/abc148b4c6e6fbe3a95f7317da1147b5ca2e23da))
- **specview:** add version management support for spec_documents ([a547803](https://github.com/specvital/worker/commit/a547803eddd022850d2eaf26ff26bd9c008e83e0))
- **specview:** implement behavior cache repository ([bea9a93](https://github.com/specvital/worker/commit/bea9a93dbd617b35f2b48bc77a45be57b674b36a))
- **specview:** implement classification cache repository ([ae3a8ba](https://github.com/specvital/worker/commit/ae3a8ba12f3d3c2bc71db772dcee878d3c0865f0))
- **specview:** implement Gemini-based AI Provider adapter ([fe5ea7d](https://github.com/specvital/worker/commit/fe5ea7dafa3619a6c6b195f5eaa8d5605d4178c0))
- **specview:** implement GenerateSpecViewUseCase for pipeline orchestration ([080978c](https://github.com/specvital/worker/commit/080978cf942463b5da339d49966d0234c20fe12e))
- **specview:** implement PostgreSQL repository for 4-table hierarchy ([e897bbf](https://github.com/specvital/worker/commit/e897bbf34c85c44ac9d0c0d6cd98804697552010))
- **specview:** implement SpecViewWorker and integrate with AnalyzerContainer ([f87e638](https://github.com/specvital/worker/commit/f87e638792c300207abbc696388fe621456b1507))
- **specview:** implement usage_events-based quota tracking ([94105c6](https://github.com/specvital/worker/commit/94105c61bb0e5fea6da9f4887348dbfdd2f0bae4))
- **specview:** integrate Phase 1 cache for incremental classification ([26a373b](https://github.com/specvital/worker/commit/26a373b9f180d8a630f41d6e1f5ab06fbfe425e2))
- **specview:** integrate Phase 2 behavior cache ([1799394](https://github.com/specvital/worker/commit/1799394d854dea2635fd173f0bb0b0beabbffe8c))
- **specview:** store user_id on spec_documents INSERT ([2ddac0d](https://github.com/specvital/worker/commit/2ddac0d487e3a62cbcdb1ee6f1baa1e6fb80bf6a))

#### 🐛 Bug Fixes

- **batch:** add debugging info for Batch API response parsing failures ([e9a6342](https://github.com/specvital/worker/commit/e9a6342b4cd6ed764163f5abf4d0b0b8e10c2f3e))
- **batch:** fix batch job repeatedly submitting instead of polling ([411e9bf](https://github.com/specvital/worker/commit/411e9bfc73212937bf4c402f46106da70c5e3426))
- **batch:** fix JSON corruption when parsing Batch API response ([93bbefb](https://github.com/specvital/worker/commit/93bbefb0d1fbf7c299cfada1dbe0acb784487fa6))
- **batch:** fix JSON parsing failure due to trailing commas in Gemini response ([cb6db9b](https://github.com/specvital/worker/commit/cb6db9b97a19b09722dae95d54548e5e9f0918f3))
- **batch:** fix parsing for Batch API split responses ([46e2302](https://github.com/specvital/worker/commit/46e2302b124133c73f5d9cae11db92d687272230))
- **gemini:** handle out-of-range test indices from AI without failing ([7d9382a](https://github.com/specvital/worker/commit/7d9382af1b5ff17c2d42562d0ac4455b48bb7d08))
- **gemini:** prevent AI hallucination causing invalid domain/feature pairs in Stage 2 assignment ([365f595](https://github.com/specvital/worker/commit/365f595ab425a046e8fbc1b82a5580d1051a6671))
- **gemini:** prevent context cancellation from truncating API responses in wave processing ([4dc421e](https://github.com/specvital/worker/commit/4dc421e38b83c8762789b731b75f6f142cd2da07))
- **gemini:** resolve taxonomy response truncation for large file sets ([0e71100](https://github.com/specvital/worker/commit/0e711005e12d159c4f6ae189db41fbe39ddf8e49))
- **prompt:** resolve Phase 2 output language mismatch with requested language ([ccea96e](https://github.com/specvital/worker/commit/ccea96e3a1ade5117220479c2d3d6bf7ce9a923b))
- **queue:** isolate dedicated queues per worker to resolve Unhandled job kind error ([3cfee6f](https://github.com/specvital/worker/commit/3cfee6fbaa5fcc1d31341d4cb9435e944cc5adfb))
- **queue:** replace colons with underscores in queue names ([0a43619](https://github.com/specvital/worker/commit/0a43619e869e3fd9ecf07566e5148d37ad8f6aa6))
- **specview:** charge quota only for AI-generated behaviors ([1cddcaa](https://github.com/specvital/worker/commit/1cddcaa5e27484c22a38327700fe846bb6457a13))
- **specview:** chunk cache not restoring on Phase 1 retry ([f4ef62d](https://github.com/specvital/worker/commit/f4ef62deb1e3418ddb0515b1ab1f2523dac0090a))
- **specview:** extend Phase 1 timeout for V3 sequential batch processing ([58bba48](https://github.com/specvital/worker/commit/58bba48ad4554924ace58d335a83c94f40d63cc6))
- **specview:** fix Batch API config not propagating to worker ([1c465c2](https://github.com/specvital/worker/commit/1c465c2cda2f229b4125ddd3e38045e2ee89e981))
- **specview:** isolate Phase 2 semaphore per job to prevent cross-job interference ([7668a98](https://github.com/specvital/worker/commit/7668a9855967367fb4bf5f0abf567efafef08945))
- **specview:** prevent behavior cache loss on Phase 2 failure ([0ec770b](https://github.com/specvital/worker/commit/0ec770bc05eca2b9207d6ba71307296f7fa5783d))
- **specview:** propagate Gemini env vars to AnalyzerContainer ([7485f08](https://github.com/specvital/worker/commit/7485f08ec2d9410b5d461cd765f545a9e5a241e4))
- **specview:** raise Phase/Job timeouts to allow large repo completion ([85089ad](https://github.com/specvital/worker/commit/85089ad6249ccc3b2c5a4ee4d826d3e7a59a3f85))
- **specview:** resolve Phase 1 response JSON truncation ([ac7d381](https://github.com/specvital/worker/commit/ac7d3819a76a6b570c2471cc2bd63199265ba3fd))
- **specview:** resolve Phase 2 output validation failures ([6c3ad64](https://github.com/specvital/worker/commit/6c3ad6403be48415acc1f9f5e22a54794bfcb8f7))

#### ⚡ Performance

- **gemini:** implement wave parallel processing to improve Phase 1 throughput ([cc33442](https://github.com/specvital/worker/commit/cc334420aafecfd15588b62f005f7e52cb37156e))
- **gemini:** reduce chunk size and enhance retry settings to minimize timeout risk ([e78ef82](https://github.com/specvital/worker/commit/e78ef82e9410036ef09a8bc3d41b176a05d04fe1))
- **gemini:** remove redundant inter-chunk delay to improve processing speed ([33a0d4d](https://github.com/specvital/worker/commit/33a0d4d8e6ffa60278cedf5ce9d6086f0c3d2926))
- **specview:** disable Gemini thinking mode to reduce Phase 1 timeout ([b47a4e9](https://github.com/specvital/worker/commit/b47a4e997abcb63923fbc25acf24c5951bf7dc50))
- **specview:** optimize prompts and timeouts to reduce Gemini 504 errors ([8857b90](https://github.com/specvital/worker/commit/8857b90e320e2a860e40f30480b26c0b4ea507bd))
- **specview:** reduce Phase 1 chunk size and add progress caching for large repositories ([7a08e01](https://github.com/specvital/worker/commit/7a08e01eaf61101e60e5329284c04ff7a2e207d2))
- **specview:** reduce Phase 1 chunk size to 500 and add JSON parse error retry ([c4edea8](https://github.com/specvital/worker/commit/c4edea8eb98ccbb22ebe27ca735e953a28abf958))

### 🔧 Maintenance

#### 📚 Documentation

- add build artifacts cleanup rule ([05e7b70](https://github.com/specvital/worker/commit/05e7b70cbeb8f3a07746bf481ce2b5641d62754c))
- add specvital-specialist agent ([9dded5d](https://github.com/specvital/worker/commit/9dded5d0e4c3610e766ee1e1bc8500eba55154c9))
- document Batch API environment variables and operation guide ([257e59b](https://github.com/specvital/worker/commit/257e59bfa797ff3b35e66f86fdca3396da868788))
- update spec-view docs ([e463e10](https://github.com/specvital/worker/commit/e463e1095a8f14e19aab40bb478dc33f1bdf8a40))

#### ♻️ Refactoring

- **app:** separate DI containers for analyzer and scheduler ([e3b1eae](https://github.com/specvital/worker/commit/e3b1eaed01306a349a75b5650886d9f1ec750e33))
- **gemini:** allow many-to-many file-feature relationship in Stage 1 taxonomy ([70bfa8f](https://github.com/specvital/worker/commit/70bfa8fa9892ee6a5e7e27deb87bacab7ae0df8a))
- **prompt:** redesign V3 classification prompt with principle-based approach ([ade4afc](https://github.com/specvital/worker/commit/ade4afc864c0c8104bcb1812e7c94fca0786c15f))
- **queue:** remove legacy queue support code ([e352f96](https://github.com/specvital/worker/commit/e352f96e083019e7a2e9eedef4f38a00e6b0310e))
- remove Scheduler service ([c163239](https://github.com/specvital/worker/commit/c163239f49c1602a285990a6f3a19b7f1c9459a3))
- **repository:** implement normalized storage using test_files table ([47cad88](https://github.com/specvital/worker/commit/47cad88accd953f2164ada321ba61f88ad791c19))
- separate queue adapter into analyze subpackage ([cf868bf](https://github.com/specvital/worker/commit/cf868bfb141a904117f6cc1ea6a722065dc3388a))
- separate worker binary into analyzer/spec-generator ([bd7f17f](https://github.com/specvital/worker/commit/bd7f17f8836d69e0c8b190f13aece43788f6413f))
- **specview:** improve Phase 2 prompt with Specification Notation style ([dbfc46d](https://github.com/specvital/worker/commit/dbfc46d30680fad6dd9f5916f24ac6422027250e))
- **specview:** improve Phase 3 prompt for user-friendly tone ([15f7e62](https://github.com/specvital/worker/commit/15f7e620753e588787d83ca9a049b33d77cbd5ec))
- **specview:** remove language constants and improve default handling ([31c1d40](https://github.com/specvital/worker/commit/31c1d409aace5dfee513dd3b7c4bac1a61644518))
- update module path and references for repository rename ([0629c97](https://github.com/specvital/worker/commit/0629c977b84525bf374ccadc14581967e8a71349))
- **worker:** separate AnalyzeWorker and SpecViewWorker into independent binaries ([f3fae45](https://github.com/specvital/worker/commit/f3fae45642a4459aed0df934891de39d0981cb1d))

#### ✅ Tests

- **gemini:** add integration tests for V3 sequential batch architecture ([30a23bc](https://github.com/specvital/worker/commit/30a23bcfbbffd484f4f7af5a3274cf4866e83d4a))
- **gemini:** implement V3 quality assurance integration tests ([79cd139](https://github.com/specvital/worker/commit/79cd139bc710321ddfa639796678e7bd54b06677))
- **gemini:** update V3 tests to expect path-based fallback instead of Uncategorized ([1964917](https://github.com/specvital/worker/commit/19649175cc0e99a550c250b5159501ce6ac445f2))
- **queue:** add integration tests for fairness middleware ([da41eea](https://github.com/specvital/worker/commit/da41eeaba9ac975ab5dca07098d4195b63271e05))
- **repository:** add integration tests for DomainHints and file_id FK relationship ([ddee5f6](https://github.com/specvital/worker/commit/ddee5f62df37ba23fc0b49138661bc0a2c99e398))
- **specview:** add E2E integration tests for SPEC-VIEW pipeline ([b803ca7](https://github.com/specvital/worker/commit/b803ca7bacc655850971501dbef7e1873070597a))

#### 🔨 Chore

- add air hot reload support for spec-generator ([c6697b2](https://github.com/specvital/worker/commit/c6697b2d8cd10ae9ffaf4017150afa731a92971d))
- add clean-containers command ([6a0d006](https://github.com/specvital/worker/commit/6a0d00608515bc5c56c57c16894420c20bf0c745))
- change license from MIT to Apache 2.0 ([393356c](https://github.com/specvital/worker/commit/393356cc417cfecd916e33aa13662ea55b5a1320))
- dumb schema ([9345080](https://github.com/specvital/worker/commit/9345080c67fd9667b65e3bd5b5b4f7594a43c588))
- dump schema ([641394d](https://github.com/specvital/worker/commit/641394d30eb4ef4f87cbd9e265f9f90199647445))
- dump schema ([ac3dd77](https://github.com/specvital/worker/commit/ac3dd77415cbadf5cd2f286e1c631bffaa27543e))
- dump schema ([5103aeb](https://github.com/specvital/worker/commit/5103aeb75f8ed56b64fdcf0f07de718358da0f57))
- dump schema ([33d8a46](https://github.com/specvital/worker/commit/33d8a46c38c8fdcfae4584b1d88705e4b658a64f))
- dump schema ([a61b1bb](https://github.com/specvital/worker/commit/a61b1bbd48b62f4f545cd84194ff8a828c509919))
- dump schema ([4450751](https://github.com/specvital/worker/commit/4450751cc76e29a0a753d10da559fa432f518013))
- dump schema ([cd3df33](https://github.com/specvital/worker/commit/cd3df33a6f74f855c1cda650516d015b88a86c3d))
- dump schema ([ea92b45](https://github.com/specvital/worker/commit/ea92b45e8c07c9d6cc13fbaa122d77fa9ae7a675))
- dump schema ([a5e6d73](https://github.com/specvital/worker/commit/a5e6d73cc218aa508d1b3d1c2c1f5655b3760d01))
- sync ai-config-toolkit ([ec52a6f](https://github.com/specvital/worker/commit/ec52a6f6eb3fb3da076414097437df03b72e53dc))
- sync docs ([8f9553b](https://github.com/specvital/worker/commit/8f9553b69ee56f7112fac89e0e42818f28ba7461))
- sync docs ([852cbfe](https://github.com/specvital/worker/commit/852cbfef6ce368fba59aeb4d6ba176cf3b17e931))
- sync-docs ([e4e8810](https://github.com/specvital/worker/commit/e4e881033d06e1d8b8a918b0d232bd83c80a3f79))
- update core ([8020c3b](https://github.com/specvital/worker/commit/8020c3b3046e8daf9f17b06f559ce2043474801f))
- update core ([dfd8e26](https://github.com/specvital/worker/commit/dfd8e26e2b9683bd180be4d624729f1742d7016e))
- update core ([a094e8d](https://github.com/specvital/worker/commit/a094e8deadd9466d8c244f8efc8e40e6d1b457b1))
- update core ([54ae38a](https://github.com/specvital/worker/commit/54ae38a02b6ec0ba1742b5ac5abd8141f26c7cad))
- update core ([ea3b03e](https://github.com/specvital/worker/commit/ea3b03e341e8538f8690b8da9ba53f327e89a3ca))
- update dev tool configs for worker→analyzer refactoring ([f1c4637](https://github.com/specvital/worker/commit/f1c4637b56e181246f5cf9e9a76fcc35d6da0f31))

## [1.1.1](https://github.com/specvital/worker/compare/v1.1.0...v1.1.1) (2026-01-04)

### 🔧 Maintenance

#### 🔨 Chore

- update core ([1f224fd](https://github.com/specvital/worker/commit/1f224fddf200ebb72e9fe11acfc114d988e21fba))

## [1.1.0](https://github.com/specvital/worker/compare/v1.0.6...v1.1.0) (2026-01-04)

### 🎯 Highlights

#### ✨ Features

- add Clone-Rename race condition detection ([ebbf443](https://github.com/specvital/worker/commit/ebbf443e3b9271c4dba82d38a567d3efdc0236a9))
- add codebase lookup queries based on external_repo_id ([d6e0b79](https://github.com/specvital/worker/commit/d6e0b797ec46aab29eb897c6c1d6a8cdbb6b47c6))
- add codebase stale handling queries and repository methods ([939e078](https://github.com/specvital/worker/commit/939e07886a8a26c385ad3206c71fe8205a6bd001))
- add GitHub API client for repository ID lookup ([46e40b8](https://github.com/specvital/worker/commit/46e40b806843f7e87ec98269baca7bce136064bd))
- determine repository visibility via reversed git ls-remote order ([0bc988e](https://github.com/specvital/worker/commit/0bc988e839f795678281ac6431b41199e4f95f95))
- integrate codebase resolution case branching into AnalyzeUseCase ([0f58440](https://github.com/specvital/worker/commit/0f58440f0012c15fd215f57a58917370ff93b2a9))
- record user analysis history on analysis completion ([e2b2095](https://github.com/specvital/worker/commit/e2b2095c47dffbd51fa57d3d24c550c19cfed851))
- store commit timestamp on analysis completion ([24bdbd7](https://github.com/specvital/worker/commit/24bdbd7050a40fbcf41965e6abfb728dc9460870))

#### 🐛 Bug Fixes

- add missing focused and xfail TestStatus types ([b24ee33](https://github.com/specvital/worker/commit/b24ee333e5c5e0f71098326f934c09853976fee6))
- add missing is_private column to test schema ([5744b95](https://github.com/specvital/worker/commit/5744b956a7b408f3dbc7583456eaedbf7fa1f4f6))
- ensure transaction atomicity for multi-step DB operations ([16834ef](https://github.com/specvital/worker/commit/16834ef0837917df4c30d31135e4f97a8a07eb3b))
- exclude stale codebases from Scheduler auto-refresh ([933c417](https://github.com/specvital/worker/commit/933c41711f375979d96cf5401ba93e6171891b49))
- fix visibility not being updated on reanalysis ([2424a5f](https://github.com/specvital/worker/commit/2424a5fadaebfd0fed1aba07045f2c86ddd5c585))
- prevent duplicate analysis job enqueue for same commit ([1a996ea](https://github.com/specvital/worker/commit/1a996ea38ad6742647317932e7acbb24939146e1))
- prevent unnecessary job retries on duplicate key error ([40eda32](https://github.com/specvital/worker/commit/40eda32b890206f3f3ef5913ce8ed4f9afdc0cdb))
- resolve stray containers from failed testcontainers cleanup ([1ef5124](https://github.com/specvital/worker/commit/1ef5124a617fcbc1ddd434b6a74baa6dd5ab390a))

#### ⚡ Performance

- improve DB save performance for large repository analysis ([200a527](https://github.com/specvital/worker/commit/200a5275cf639a2c0c65d955e79dbe65ad4f7068))

### 🔧 Maintenance

#### 🔧 Internal Fixes

- **devcontainer:** fix network creation failure in Codespaces ([2054227](https://github.com/specvital/worker/commit/2054227927b13127fb2c770323dcc17e6bba4d0a))
- isolate git ls-remote environment to fix private repo misclassification ([7d15fb8](https://github.com/specvital/worker/commit/7d15fb82534cb2c4c34ea368173265c185abf543))

#### 📚 Documentation

- add CLAUDE.md ([5194d71](https://github.com/specvital/worker/commit/5194d713b2f07fd2d4d2a66df62f861520b027bc))
- add missing version headers and improve CHANGELOG hierarchy ([d6436ab](https://github.com/specvital/worker/commit/d6436ab60b12e4bf551c23d59009fa66782e6eb4))
- rename infra repo in docs ([1bdb806](https://github.com/specvital/worker/commit/1bdb806dabc7fd082cb114e93f349aaa619d5315))

#### 💄 Styles

- format code ([8616fbd](https://github.com/specvital/worker/commit/8616fbdae4105860c87569093f302ba6a877c6c7))

#### ♻️ Refactoring

- remove unused deprecated Stop method ([c034ecc](https://github.com/specvital/worker/commit/c034ecc56660bda965a297072e7d23400e8b8e61))
- **test:** auto-sync test schema with production schema ([77668e0](https://github.com/specvital/worker/commit/77668e0e946003dc4f0d3b9e9c086c85b70f8fab))

#### 🔨 Chore

- changing the environment variable name for accessing GitHub MCP ([553c63d](https://github.com/specvital/worker/commit/553c63d358a5b1fd1c607843d41b90544d86330e))
- dump schema ([ba3fc16](https://github.com/specvital/worker/commit/ba3fc165a074f0827417ee6212002e79c9d5340e))
- dump schema ([425b609](https://github.com/specvital/worker/commit/425b6098dc1ee104189a4a33dc635f5e0b9f0352))
- dump schema ([52575e5](https://github.com/specvital/worker/commit/52575e5701088de44401abb227080800250094d8))
- dump schema ([abdaa2e](https://github.com/specvital/worker/commit/abdaa2eda93763d793b2a8a67f6fe2f3b4e14166))
- fix vscode import area not automatically collapsing ([ac92e87](https://github.com/specvital/worker/commit/ac92e87ee1be68a886e4df8b5ed006d0fc8ba0dd))
- improved the claude code status line to display the correct context window size. ([e1fa775](https://github.com/specvital/worker/commit/e1fa775b9dfd49ed57ec5d66aaf0eab4ec0e34b8))
- modified container structure to support codespaces ([0d1fec6](https://github.com/specvital/worker/commit/0d1fec6ec9af2bd3fb1df5a292242e240e13a36e))
- modify local db migration to always initialize the database ([7709a5b](https://github.com/specvital/worker/commit/7709a5b8af0a8fd7bee795ebd533dd5d3944d243))
- sync ai-config-toolkit ([0d00d4a](https://github.com/specvital/worker/commit/0d00d4a615fa3b1c162e8976b0f86b87948f0eaf))
- sync docs ([86772da](https://github.com/specvital/worker/commit/86772da7cb514400b7f7c89ea0defde95241195e))
- update core ([9092761](https://github.com/specvital/worker/commit/9092761f54e28b114b70a7dfbab14e8b82e27bdc))
- update core ([e6613c3](https://github.com/specvital/worker/commit/e6613c3a8e85189621056981ae0e3d91ff266e41))
- update core ([c163ae9](https://github.com/specvital/worker/commit/c163ae92f08d30046712de8c4b86b3162eaae758))

## [1.0.6](https://github.com/specvital/worker/compare/v1.0.5...v1.0.6) (2025-12-19)

### 🎯 Highlights

#### 🐛 Bug Fixes

- resolve 60-second timeout failure during large analysis jobs ([ed18bc3](https://github.com/specvital/worker/commit/ed18bc3f587c0a446ea41b09431126ab1d22bba5))

## [1.0.5](https://github.com/specvital/worker/compare/v1.0.4...v1.0.5) (2025-12-19)

### 🎯 Highlights

#### 🐛 Bug Fixes

- resolve 60-second timeout issue during bulk INSERT operations on NeonDB ([0b6bc9b](https://github.com/specvital/worker/commit/0b6bc9bbef0c14190ec953a14caa5b29da0422d5))

## [1.0.4](https://github.com/specvital/worker/compare/v1.0.3...v1.0.4) (2025-12-19)

### 🔧 Maintenance

#### ♻️ Refactoring

- migrate queue system from asynq(Redis) to river(PostgreSQL) ([9664002](https://github.com/specvital/worker/commit/9664002057ef1f801dd8313e9f081760c3e0af21))

#### 🔨 Chore

- missing changes ([de9c0ec](https://github.com/specvital/worker/commit/de9c0ecaa7e136fe71d70a4a231dbd194ed7a33d))

## [1.0.3](https://github.com/specvital/worker/compare/v1.0.2...v1.0.3) (2025-12-18)

### 🎯 Highlights

#### 🐛 Bug Fixes

- fix git clone failure in runtime container ([f11dfa3](https://github.com/specvital/worker/commit/f11dfa3e4090a6412af71b58a7eca6f081e49d4d))

### 🔧 Maintenance

#### ♻️ Refactoring

- remove unused dead code ([95cee17](https://github.com/specvital/worker/commit/95cee17e1307fe3e0cc23ba7d549292b05c19744))

#### 🔨 Chore

- sync docs ([9007e97](https://github.com/specvital/worker/commit/9007e97bbff365afcae69cfcba2f501732a20c8b))

## [1.0.2](https://github.com/specvital/worker/compare/v1.0.1...v1.0.2) (2025-12-17)

### 🔧 Maintenance

#### 🔧 Internal Fixes

- fix asynq logs incorrectly classified as error in Railway ([d2180cc](https://github.com/specvital/worker/commit/d2180cc1182a0f1187f7dd63b982fc7816e3be47))

## [1.0.1](https://github.com/specvital/worker/compare/v1.0.0...v1.0.1) (2025-12-17)

### 🎯 Highlights

#### 🐛 Bug Fixes

- enable CGO for go-tree-sitter build ([50b1fea](https://github.com/specvital/worker/commit/50b1fea3c7834bd585c3a23615d8acb5cbae8a5f))

## [1.0.0](https://github.com/specvital/worker/releases/tag/v1.0.0) (2025-12-17)

### 🎯 Highlights

#### ✨ Features

- add adaptive decay logic for auto-refresh scheduling ([8a85854](https://github.com/specvital/worker/commit/8a858547c7b8a5190176253830b862588cda8042))
- add enqueue CLI tool ([5697cb9](https://github.com/specvital/worker/commit/5697cb9533b8dcfd8c7c90fa34d5419b181fa287))
- add focused/xfail to test_status and support modifier column ([cd60233](https://github.com/specvital/worker/commit/cd602333aa9eba34e8a3b21bbcd91040bfe59936))
- add job timeout to prevent long-running analysis jobs ([392b43e](https://github.com/specvital/worker/commit/392b43e7395aa923fd05a96872e5f0c8911a8845))
- add local development mode support to justfile ([2ca2d51](https://github.com/specvital/worker/commit/2ca2d51337cd8c425e54a5f3ffd257bde28e403c))
- add local development services to devcontainer ([a30ca9e](https://github.com/specvital/worker/commit/a30ca9eca3df2aa4a418d94b1525f8ac170ee6c1))
- add OAuth token parameter to VCS Clone interface ([de518d0](https://github.com/specvital/worker/commit/de518d05eb534c97cf5e647168ff9d8a686ae00e))
- add semaphore to limit concurrent git clones ([9ddbc06](https://github.com/specvital/worker/commit/9ddbc06291c8fb41e0e39def7f32edaa5830f1ba))
- add UserRepository for OAuth token lookup ([9a16ec1](https://github.com/specvital/worker/commit/9a16ec1f37031d15612198b27f5316d4ab066225))
- implement analysis pipeline (git clone → parse → DB save) ([66dd262](https://github.com/specvital/worker/commit/66dd2627b6e7d5b46ab7bc4358a1f6178d77cfee))
- implement asynq-based worker basic structure ([4dd16ad](https://github.com/specvital/worker/commit/4dd16ad22b355229f6c9db12e076427f7ff0c2ea))
- initialize collector service project ([1d3c8cf](https://github.com/specvital/worker/commit/1d3c8cf3a570a719301fa953c9cccb9c53a0358a))
- integrate OAuth token lookup logic into UseCase ([cb3f911](https://github.com/specvital/worker/commit/cb3f9114f89b9435f037379b6eac07c51dc53d96))
- integrate scheduler for automatic codebase refresh ([e0a1a15](https://github.com/specvital/worker/commit/e0a1a15dac6ef26b62debf1d3b52d172cf8d8ed6))
- record failure status in DB when analysis fails ([6485ac3](https://github.com/specvital/worker/commit/6485ac345c9562404e435ea32962a1b90a13fd5b))
- support external analysis ID for record creation ([5448202](https://github.com/specvital/worker/commit/54482021c84b6faa466f0ce006de06ddcd79d22d))
- support OAuth token decryption for private repo analysis ([8d0ad30](https://github.com/specvital/worker/commit/8d0ad307ce07aa7562daa6a38230e19ce2cc1644))

#### 🐛 Bug Fixes

- handle missing error logging and DB status update on analysis task failure ([64ae8d9](https://github.com/specvital/worker/commit/64ae8d9fcbc620e7470e2c7cc21ade39d7327f8d))
- parser scan failing due to unexported method type assertion ([6256673](https://github.com/specvital/worker/commit/6256673dc23d48998851f977fcd034498c591642))
- remove unnecessary wait and potential deadlock in graceful shutdown ([b78c981](https://github.com/specvital/worker/commit/b78c981c662d32fde66c0890da0e226e9b4a4d3e))

### 🔧 Maintenance

#### 🔧 Internal Fixes

- go mod tidy ([c58f73b](https://github.com/specvital/worker/commit/c58f73b40f2de49c8c69b1d67efd45b1487c0359))

#### 💄 Styles

- format code ([5e994e2](https://github.com/specvital/worker/commit/5e994e2ab90f6ae6a8cd64d392b946c9bde0bd1d))

#### ♻️ Refactoring

- centralize dependency wiring with DI container ([c1b8215](https://github.com/specvital/worker/commit/c1b82151bdba8b62e194100e6f04271fd3f4e026))
- extract domain layer with zero infrastructure dependencies ([7ba9e51](https://github.com/specvital/worker/commit/7ba9e51a0ffa76327736e91c828fe5949cbfbcb6))
- extract repository layer from AnalyzeHandler ([464ecfa](https://github.com/specvital/worker/commit/464ecfa6087d91d4d399a7fee032ed2a9109a151))
- extract service layer from AnalyzeHandler ([d9faf20](https://github.com/specvital/worker/commit/d9faf200da77b3097417cd1671a3a1c5fbc5fe06))
- implement handler layer and clean up legacy packages ([23a093f](https://github.com/specvital/worker/commit/23a093f6d5558d10decb21272289c1d99e583101))
- implement repository adapter layer (Clean Architecture Commit 3) ([8b0e433](https://github.com/specvital/worker/commit/8b0e43372fec1b15fa7e7d76794be37d42b6988e))
- implement use case layer with dependency injection ([b2be6ff](https://github.com/specvital/worker/commit/b2be6ff3d3a1c3662f816958d614f4a89215aba6))
- implement VCS and parser adapter layer (Clean Architecture Commit 4) ([1b2e34f](https://github.com/specvital/worker/commit/1b2e34f61665d6513c2654712390b67524dfd731))
- move infrastructure packages to internal/infra ([6cc1d1c](https://github.com/specvital/worker/commit/6cc1d1caf8722317967eae5c2de6c40c71467ce2))
- separate Scheduler from Worker into independent service ([9481141](https://github.com/specvital/worker/commit/9481141e99f0adcc225e93d05c8104846f836c17))
- split entry points for Railway separate deployments ([d899192](https://github.com/specvital/worker/commit/d899192cb1772fe9ed16d426d460e016c1bbf2ee))

#### ✅ Tests

- add AnalyzeHandler unit tests ([0286e7c](https://github.com/specvital/worker/commit/0286e7cd687d65be853e503a883cd74010f8dede))
- remove unnecessary skipped tests ([f8c0eb4](https://github.com/specvital/worker/commit/f8c0eb40a4f99252650bbf9e0f9ca93a378223fb))

#### 🔧 CI/CD

- configure semantic-release automated deployment pipeline ([37f128f](https://github.com/specvital/worker/commit/37f128f2c9d113144d2af530e88f84d3209f235c))

#### 🔨 Chore

- add bootstrap command ([c8371f0](https://github.com/specvital/worker/commit/c8371f0d5f47a19353c260ecf83c5033b4e5ba53))
- add Dockerfile for collector service ([6e3b0e4](https://github.com/specvital/worker/commit/6e3b0e4225b4b0875e2ee0bae909a594b1b9f87c))
- add example env file ([64a24a4](https://github.com/specvital/worker/commit/64a24a4de88aa5e4954a59b049b915c2012da79e))
- add gitignore item ([8fc64a6](https://github.com/specvital/worker/commit/8fc64a6ab0a3abca1cf1d73f458adf17bb752ced))
- add migrate local command ([baabcfe](https://github.com/specvital/worker/commit/baabcfe97f3905122026a752ea2ba7f7ed07917b))
- add PostgreSQL connection and sqlc configuration ([eecc4a6](https://github.com/specvital/worker/commit/eecc4a69a8b8c6e5c67a8f652a97ae784ecca1c1))
- add useful action buttons ([02fa778](https://github.com/specvital/worker/commit/02fa7785ac4ba1505e06ac3add60621cf01d1be9))
- adding recommended extensions ([30d5d0b](https://github.com/specvital/worker/commit/30d5d0b0fccc3190313456433e24e1342c18d641))
- ai-config-toolkit sync ([3091cf4](https://github.com/specvital/worker/commit/3091cf46ca2e6a24f5c299fe4f8008659fe1b8c8))
- ai-config-toolkit sync ([decf96b](https://github.com/specvital/worker/commit/decf96b2c47b278ef56a3e76c6174c9688f883c3))
- delete file ([f48005c](https://github.com/specvital/worker/commit/f48005cad322fa2586e9bf315e2bce3c608dcd8b))
- dump schema ([b90bab0](https://github.com/specvital/worker/commit/b90bab0f0c33d52683b6ff6a1f132702eb54a077))
- dump schema ([370409c](https://github.com/specvital/worker/commit/370409cee67512bc3f21ac3f5835357303db9b57))
- dump schema ([d704305](https://github.com/specvital/worker/commit/d7043054a0a59ac755bc23a85a4fd39f5ce97a0a))
- Global document synchronization ([cead255](https://github.com/specvital/worker/commit/cead255f25f48397848d55cf1417f21466dae67c))
- sync ai-config-toolkit ([e559889](https://github.com/specvital/worker/commit/e55988903526ade4630d2d6516e67ad1354ff67e))
- update core ([d358131](https://github.com/specvital/worker/commit/d358131e3e6197ee8958655b3cc1cfa7d0ed9ca6))
- update core ([b47592e](https://github.com/specvital/worker/commit/b47592e2a6668c25585d0338099e83e7b72bf1d5))
- update core ([395930a](https://github.com/specvital/worker/commit/395930a21bb48b8283cac037cde1999e44ae69c6))
- update schema.sql path in justfile ([0bcbe79](https://github.com/specvital/worker/commit/0bcbe794cdbccff58e2babe75a6308aacc6ad5d0))
- update-core version ([cc65b03](https://github.com/specvital/worker/commit/cc65b0325a1e828e24270753d76fa91ff01eeb45))
